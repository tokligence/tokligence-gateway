package responses

import (
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Conversation captures the canonical payload exchanged between ingress, orchestrator,
// and provider layers for the Responses API. It bundles the original Responses request
// (Base) with the translated OpenAI ChatCompletionRequest (Chat).
type Conversation struct {
	ID   string
	Base openai.ResponseRequest
	Chat openai.ChatCompletionRequest
}

// NewConversation builds a Conversation from the provided Responses and Chat payloads.
// The chat request is cloned to ensure subsequent mutations do not leak across layers.
func NewConversation(base openai.ResponseRequest, chat openai.ChatCompletionRequest) Conversation {
	return Conversation{
		ID:   strings.TrimSpace(base.ID),
		Base: base,
		Chat: CloneChatCompletionRequest(chat),
	}
}

// WithChat returns a copy of the conversation with the Chat payload replaced.
func (c Conversation) WithChat(chat openai.ChatCompletionRequest) Conversation {
	c.Chat = CloneChatCompletionRequest(chat)
	return c
}

// WithBase returns a copy of the conversation with the base Responses payload replaced.
func (c Conversation) WithBase(base openai.ResponseRequest) Conversation {
	c.Base = base
	if id := strings.TrimSpace(base.ID); id != "" {
		c.ID = id
	}
	return c
}

// CloneChat exposes a safe copy of the underlying chat payload.
func (c Conversation) CloneChat() openai.ChatCompletionRequest {
	return CloneChatCompletionRequest(c.Chat)
}

// StructuredJSON reports whether the conversation expects structured JSON output.
func (c Conversation) StructuredJSON() bool {
	format := strings.TrimSpace(c.Base.ResponseFormat.Type)
	return strings.EqualFold(format, "json_object") || strings.EqualFold(format, "json_schema")
}

// EnsureID guarantees the conversation has a stable ID value.
func (c Conversation) EnsureID(id string) Conversation {
	id = strings.TrimSpace(id)
	if id == "" {
		return c
	}
	c.ID = id
	c.Base.ID = id
	return c
}

// CloneChatCompletionRequest deep-copies the provided chat completion request so that
// slices (messages, tools, tool calls) are not shared across sessions.
func CloneChatCompletionRequest(src openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	dst := src
	if len(src.Messages) > 0 {
		dst.Messages = make([]openai.ChatMessage, len(src.Messages))
		for i, msg := range src.Messages {
			dst.Messages[i] = cloneChatMessage(msg)
		}
	}
	if len(src.Tools) > 0 {
		dst.Tools = append([]openai.Tool(nil), src.Tools...)
	}
	return dst
}

func cloneChatMessage(msg openai.ChatMessage) openai.ChatMessage {
	out := msg
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			tcCopy := tc
			out.ToolCalls[i] = tcCopy
		}
	}
	return out
}
