# OpenAI ↔ Anthropic API Mapping

Tokligence Gateway 旨在在 OpenAI 兼容接口与 Anthropic Claude 接口之间提供双向翻译。本文档梳理了核心端点、请求/响应字段、流式行为和错误语义，便于实现 `openai-to-anthropic` 与 `anthropic-to-openai` 适配器。

## 1. 核心端点
| 功能 | OpenAI | Anthropic | 说明 |
| --- | --- | --- | --- |
| Chat Completion | `POST /v1/chat/completions` | `POST /v1/messages` | 双方都支持多轮对话；Anthropic 将 system 指令通过 `system` 字段或作为 `messages[0]` 传递。 |
| Text Completion *(legacy)* | `POST /v1/completions` | `POST /v1/messages` | Claude 仅提供 message 模型，可通过构造单轮 user 输入兼容。 |
| Streaming Chat | `POST /v1/chat/completions` + `stream=true` | `POST /v1/messages` + `stream=true` | 均采用 SSE；事件名称与 payload 字段不同，需转换为 OpenAI 样式。 |
| Tool Calls / Function Calling | `tool_calls` 字段（`type=json_schema` 等） | `content` 数组中的 `tool_result` / `tool_use` 片段 | 需在 Adapter 中双向映射。 |
| Embeddings | `POST /v1/embeddings` | `POST /v1/embeddings` | Claude 仅个别模型支持；响应格式需在维度与 token 计费上对齐。 |
| Model 列表 | `GET /v1/models` | `GET /v1/models` | 需对模型 ID 做映射（如 `gpt-4o-mini` ↔ `claude-3-5-sonnet-20241022`）。 |
| Usage 统计 | 响应 `usage.prompt_tokens` 等 | 响应 `usage.input_tokens`, `usage.output_tokens` | 字段数量与命名不同，需标准化为 OpenAI 风格。 |
| Moderation | `POST /v1/moderations` | - | Claude 暂无等价接口，可返回 `not_implemented`。 |

## 2. 请求字段映射
### 2.1 消息结构
OpenAI `messages` 数组示例:
```json
{"role":"system","content":"You are helpful"}
{"role":"user","content":[{"type":"text","text":"hi"}]}
{"role":"assistant","content":[{"type":"tool_call","id":"call_1","function":{"name":"lookup","arguments":"{...}"}}]}
```
Anthropic `messages` 采用交替的 `role` 与 `content`：
```json
{"role":"user","content":[{"type":"text","text":"hi"}] }
{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"lookup","input":{...}}]}
```
映射策略：
- `system` 提示：优先使用 Anthropic 请求体的 `system` 字段；缺省时将 system 消息插入 `messages` 首位。
- `assistant` tool call：OpenAI 的 `tool_calls[i].function.arguments` 为 JSON 字符串；Anthropic 需要 `tool_use.input` 为对象。Adapter 应在入站时解析 JSON，在出站时重新编码。
- `tool` / `tool_result`：OpenAI 的 `messages[i].role="tool"` 转换为 Anthropic `assistant` 消息中的 `tool_result` 片段，字段 `tool_use_id` 对应调用 ID。
- 多模态：OpenAI 的 `content` 可包含 `image_url`; Anthropic Claude 3 支持 `image` content，需转换为 base64 或引用 URL。

### 2.2 采样参数
| OpenAI 字段 | Anthropic 字段 | 备注 |
| --- | --- | --- |
| `model` | `model` | 需维护模型别名表。 |
| `temperature` | `temperature` | 数值范围一致 (0-2)。 |
| `top_p` | `top_p` | Anthropic 默认 0.999；支持。 |
| `top_k` | `top_k` | OpenAI 无该字段，向下兼容时可忽略。 |
| `frequency_penalty` | - | Claude 暂不支持，需在适配器中丢弃或模拟。 |
| `presence_penalty` | - | 同上。 |
| `stop` | `stop_sequences` | OpenAI 接受字符串或数组；需统一为数组。 |
| `max_tokens` | `max_output_tokens` | 注意 Claude 限制常以输入+输出计费。 |
| `response_format` | `metadata.json_mode=true` | Claude 通过 metadata 标志强制 JSON 模式。 |

### 2.3 工具声明
OpenAI `tools` (JSON schema) 转 Anthropic `tools`：
- `type:"function"` → `type:"tool"`, `input_schema` 保持 JSON Schema 结构。
- 支持 `strict=true` 时，将 `metadata.strict=true`。
- Anthropic 需要 `tools` 数组中的每个条目包含 `name`,`description`,`input_schema`。

## 3. 响应字段映射
| OpenAI 响应字段 | Anthropic 字段 | 适配说明 |
| --- | --- | --- |
| `choices[i].message.role` | `content` 最后一段的角色 | 需从 Claude 响应构造最终消息。 |
| `choices[i].message.content` | `content` 中 `text`/`tool_use` 片段 | 将 Claude 的多段内容合并为 OpenAI 的 `content` 数组。 |
| `choices[i].finish_reason` | `stop_reason` | 保留 `stop`, `length`, `tool_use`, `max_tokens`. |
| `usage.prompt_tokens` | `usage.input_tokens` | 映射字段名。 |
| `usage.completion_tokens` | `usage.output_tokens` | -- |
| `usage.total_tokens` | `input_tokens + output_tokens` | 需在适配器中计算。 |

## 4. SSE 流式转换
OpenAI SSE 事件序列：`event: message`, `event: ping`, `event: error`, `event: done`。
Anthropic SSE 发送 `event: message_start`, `event: content_block_delta`, `event: message_delta`, `event: message_stop`。
适配器需：
1. 将 `message_start` 转换为带空 `delta` 的 OpenAI `event: message`。
2. `content_block_delta` → `delta.content[0].text`.
3. `message_delta` 中的 `stop_reason` → `finish_reason`.
4. `message_stop` → `event: done`。

## 5. 错误语义
| OpenAI 错误 | Anthropic 错误 | 适配策略 |
| --- | --- | --- |
| `400` + `invalid_request_error` | `400` + `invalid_request_error` | 传递 `message`，保持 `type`. |
| `401` + `authentication_error` | `401` + `authentication_error` | 直接映射。 |
| `429` + `rate_limit_error` | `429` + `rate_limit_error` | 归一为 OpenAI `rate_limit`。 |
| `500` | `500` | 包装为 `server_error`. |
| `error.code` | `error.type`/`error.code` | 保留 Claude 的 `type` 作为扩展。 |

## 6. Token 计费差异
- OpenAI 以 prompt/completion 分别计费；Anthropic 报告 `input_tokens` 与 `output_tokens`。
- Claude 针对不同模型的 token 单价差异较大，需要在 Token Accounting 中维护 `model_family` → `pricing`。
- Claude 支持 `cache_creation_input_tokens` 等缓存字段，暂可忽略或记录在扩展字段。

## 7. 参考实现要点
- 适配器实现两个方向：
  1. **OpenAI → Anthropic**：在 Tokligence Gateway 内，将 OpenAI SDK 风格请求转换为 Claude API 调用，供 `openai-to-anthropic` 使用。
  2. **Anthropic → OpenAI**：为需要使用 Claude 客户端访问 Gateway 的用户提供 OpenAI 兼容响应。
- 维护共享的模型元数据表（模型名称、上下文窗口、速率限制、默认温度等），供 CLI/前端展示。
- 在单元测试中覆盖：消息包含 tool_call、JSON 模式、流式片段、错误映射。

## 8. 后续扩展
- 添加 Claude Prompt Caching (`anthropic-beta:cache-control`).
- 支持 Claude Audio/Multimodal (`modalities`).
- 研究 OpenAI Reponses API ↔ Claude Workflows 映射。
