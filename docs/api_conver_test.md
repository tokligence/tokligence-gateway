# Claude Code → Gateway → OpenAI 互通（SSE）测试手册

本手册指导你把 Claude Code 接到 Tokligence Gateway，并通过 OpenAI 完成来回对话（SSE 输出给 Claude Code，上游到 OpenAI 可选择批量或流式）。

## 准备
- 依赖：Go 1.21+（已 vendored 依赖）、GNU Make。
- OpenAI 凭证：导出 `TOKLIGENCE_OPENAI_API_KEY`（必需）。如果用自建兼容端点，设置 `TOKLIGENCE_OPENAI_BASE_URL`。
- 配置位于 `config/setting.ini` + `config/dev/gateway.ini`，本仓库已默认：
  - `log_level=debug`（默认 DEBUG 日志）
  - `log_file_daemon=logs/dev-gatewayd.log`（日志写入，自动每天滚动）
  - `auth_disabled=true`（全局关闭 Authorization 验证，便于开发）
  - `anthropic_native_enabled=true`（默认）
  - `anthropic_force_sse=true`（默认；Claude Code 只收 SSE）
  - `openai_tool_bridge_stream=false`（默认；上游 OpenAI 走批量，网关对外仍以 SSE 推送）

## 构建与启动
1) 构建守护进程：
   - `make build-gatewayd`
   - 生成二进制 `bin/gatewayd`

2) 启动网关：
   - `TOKLIGENCE_OPENAI_API_KEY=sk-xxx bin/gatewayd`
   - 终端会看到前缀 `[gatewayd/http]` 的 DEBUG 日志。
   - 日志文件写入 `logs/dev-gatewayd.log`（符号链接），指向当日文件 `logs/dev-gatewayd-YYYY-MM-DD.log`，使用 UTC 日期滚动。
   - 查看日志：`tail -f logs/dev-gatewayd.log`

## 自检（curl）
1) count_tokens（Claude Code 会先调用）：
   - `curl -s http://localhost:8081/anthropic/v1/messages/count_tokens -H 'content-type: application/json' -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}'`
   - 预期 200，返回 `{"input_tokens": N}`。

2) SSE 消息接口（无工具，直通翻译）：
   - `curl -N http://localhost:8081/anthropic/v1/messages -H 'content-type: application/json' -d '{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"Say hi"}]}'`
   - 预期事件序列：`message_start` → `content_block_start(text)` → `content_block_delta`* → `content_block_stop` → `message_delta` → `message_stop`。

3) 工具桥（上游 OpenAI，默认批量、对外 SSE）：
   - 构造包含 `tools` 或 `tool_*` 内容块的请求，Claude Code 会收到规范的 Anthropic SSE 事件。
   - 若要改为上游 OpenAI 也走流式：设置 `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM=true` 再启动。

## 将 Claude Code 指向网关
- 目标端点（Anthropic 兼容）：`http://localhost:8081/anthropic/v1/messages`
- Claude Code 只接受 SSE，网关已默认 `anthropic_force_sse=true`。
- 授权：默认关闭（`auth_disabled=true`），无需 `Authorization` 头。若你开启了授权，请在 Claude Code 里设置 `Authorization: Bearer <你的本地 API Key>`。
- Claude Code 可能带 `?beta=true`，网关兼容。

## 排错
- 401 invalid token：确认 `auth_disabled=true`（默认）。若开启授权，需要在本地用户库创建 API Key 并提供 `Authorization`。
- 404 /messages/count_tokens：已支持该路由；若仍见 404，确认 `anthropic_native_enabled=true` 与路由路径无误。
- JS 报错 `H.map undefined`：通常为 SSE 事件序列不完整。当前网关已修复流式桥接的事件顺序；请升级并确认上游/对外均使用上述事件序列。
- 日志未写文件：确认 `config/dev/gateway.ini` 的 `log_file_daemon=logs/dev-gatewayd.log` 未被环境变量覆盖；日志按 UTC 日期滚动至 `logs/dev-gatewayd-YYYY-MM-DD.log`，可跟随 `logs/dev-gatewayd.log` 符号链接查看。

## 备注
- 网关会对 Claude Code 强制 SSE 输出；上游到 OpenAI 可按需选择批量或流式（由 `openai_tool_bridge_stream` 控制）。
- token 守卫（`anthropic_token_check_enabled`）默认关闭；开启后需提供 `max_tokens` 并受 `anthropic_max_tokens` 限制。

