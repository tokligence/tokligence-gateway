# Anthropic (Claude Code) → OpenAI Tool Bridge: Adapter Design

This document explains how the gateway translates Anthropic’s Claude Code protocol (the native `/anthropic/v1/messages` API) into OpenAI’s Chat Completions + Tools API. It covers routing, normalization, message mapping, streaming, duplicate‑suppression, configuration, and a suggested validation plan.

## Purpose
- Allow Claude Code (Anthropic client) to talk to the gateway while the gateway calls OpenAI upstreams.
- Preserve tool workflows end‑to‑end (tool_use → tool_result → final answer) with minimal surprises.
- Prevent feedback loops such as repeated “Read README” tool calls caused by history replay.

## Preconditions
- Routing resolves the incoming model to the `openai` adapter.
  - Suggested default: `claude*` → `openai` when Anthropic is not configured; otherwise `claude*` → `anthropic`.
- OpenAI API key configured.
- Anthropic‑native endpoint enabled (default on) so Claude Code clients can call `/anthropic/v1/messages`.

## High‑Level Flow
1. Receive Anthropic request (may include `system`, `messages`, `tools`, `stream`).
2. Normalize to a canonical internal form (blocks array for content).
3. Build an OpenAI request with mapped messages and tools.
4. Call OpenAI (non‑stream or stream) and map the response back to Anthropic format.
5. Record usage in the ledger.

## Mapping Rules (R1–R10)

- R1 Normalize input content
  - Coerce `message.content` into a canonical blocks array:
    - string → `[{type:text, text:...}]`
    - `{text:string}` → `[{type:text, text:...}]`
    - `{content:string}` → `[{type:text, text:...}]`
    - `{content:[blocks]}` → `[blocks]`

- R2 Merge system text
  - Extract `system` text (string or blocks). Remove internal hints like `<system-reminder>…</system-reminder>` before sending upstream.

- R3 Declare tools and set tool choice conservatively
  - Build OpenAI `tools` schema from Anthropic `tools`.
  - Choose `tool_choice` dynamically:
    - If the current turn contains any `tool_result` blocks → `tool_choice:"none"` (generate text, no more tools now).
    - Else if tools are declared → `tool_choice:"auto"` (model may call tools if needed).
    - Else omit `tool_choice`.

- R4 Assistant tool_use → OpenAI assistant.tool_calls
  - Aggregate `tool_use` blocks in an assistant turn into a single OpenAI assistant message with `tool_calls`.
  - If the assistant turn also includes text, include it in the same assistant message.

- R5 User tool_result → OpenAI tool message
  - Map each `tool_result` block to an OpenAI `role:"tool"` message.
  - Set `tool_call_id` to the original `tool_use.id` when present.

- R6 Split tool_result and user text safely
  - If a single Anthropic user turn contains both tool_result and text, forward the tool_result as `tool` messages first.
  - Only add user text when it is genuinely a new instruction (see R8).

- R7 History replay safety
  - Many clients resend full history each turn. The adapter must not re‑trigger the same tool from replayed text.

- R8 Text de‑duplication
  - Within a turn: if any `tool_result` exists, do not append the turn’s plain text as a new user message.
  - Across turns: track the last emitted user text; do not forward an identical user text again.

- R9 Streaming bridging
  - When `stream=true`, forward OpenAI SSE deltas to Anthropic style:
    - Text deltas → `content_block_delta` events.
    - Tool discovery turns (tool_calls only) produce no text deltas; after tool_result, assistant text deltas stream normally.

- R10 System guardrail
  - Inject a small system instruction: “If history already contains a matching tool response for identical arguments, don’t repeat the same tool call unless the user explicitly asks to refresh.” This reduces loops in agent workflows.

### Read‑Only Intent Policy (Optional but Recommended)

- Detect common “read/explain/summarize” intents from the latest user text (e.g., contains: read/readme/summary/summarize/解释/总结; and not contains: edit/update/修改/写).
- When intent is read‑only, filter the outbound OpenAI `tools` to a safe allowlist (e.g., Glob, Read, Grep, LS, NotebookRead, WebFetch, WebSearch) so write tools (Edit/Write/MultiEdit/TodoWrite/ApplyPatch) are not available to the model.
- Combined with `tool_choice:"none"` after a `tool_result` turn, this prevents unwanted write operations during summarization/explanation tasks.

## Error Tolerance
- Accepts flexible shapes for `tool_result.content`: string or block array.
- For streaming, usage is approximated when upstream doesn’t report final counts.
- Conservative stop‑reason mapping to keep behavior predictable.

## Configuration
- Route selection determines whether the bridge is used:
  - If route resolves to `openai` and OpenAI key is set: bridge is active.
  - If route resolves to `anthropic` and passthrough enabled: proxy to Anthropic.
- Streaming toggle for tool bridge:
  - `openai_tool_bridge_stream` (env: `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM`, default false).
- Debug level for detailed logs (`log_level=debug`).

## Diagnostics (Log Tags)
- `anthropic.raw` / `anthropic.normalize` – input preview and normalized payload.
- `anthropic.messages` – route decision and tool bridge usage.
- `openai.bridge` / `openai.bridge(stream)` – upstream request, status, and tool_calls count.
- `tool_use …` / `tool_result …` – brief previews of tool args and results for troubleshooting.

## Validation Plan (Experiments)

- E1 Minimal tool chain
  - User: “Read README and summarize.” Tools: Glob, Read.
  - Expect: first turn → tool_calls; client returns `tool_result`; second turn → assistant text. No repeated Read.

- E2 Mixed turn (tool_result + text in same user turn)
  - Expect: bridge forwards only tool_result as `tool` messages; does not add the duplicate user text; no repeated tool call.

- E3 History replay
  - Client resends history including the same user text and prior tool_result.
  - Expect: duplicate user text is filtered; no new tool call unless instruction changed.

- E4 Intentional refresh
  - User explicitly asks to “re‑read README”.
  - Expect: new Read tool call (refresh is allowed by R10 guardrail wording).

- E5 Streaming
  - With `stream=true`, confirm: tool discovery turns may have no deltas; after `tool_result`, assistant text deltas stream as Anthropic `content_block_delta`.

## Known Limitations and Next Steps
- Near‑duplicate text (minor punctuation/whitespace differences) may bypass de‑duplication; consider a fuzzy match if needed.
- Some orchestrators embed heavy “system reminders” inside user text; continue expanding `stripSystemReminder` if new patterns emerge.
- Optional: add a “soft threshold” policy—if upstream repeats identical tool_calls when a matching tool_result is already present, retry once with an added instruction to avoid repetition.

---

This design aims to be stable across tool‑heavy coding workflows while staying faithful to both protocols. If you see repeated tool calls in logs, capture the surrounding `anthropic.messages` and `openai.bridge` snippets; these reveal whether a duplicate user text or mixed turn caused the loop.
