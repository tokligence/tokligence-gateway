package httpserver

import (
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// buildResponsesResponseFromChat maps a chat completion response into the lightweight
// OpenAI Responses shape used by the gateway facade. It preserves output_text,
// tool call metadata, refusal markers, and token usage.
func buildResponsesResponseFromChat(creq openai.ChatCompletionRequest, resp openai.ChatCompletionResponse) responsesResponse {
	var outText string
	var toolBlocks []map[string]interface{}
	var refusalBlock map[string]interface{}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		// Extract content as string (supports string and structured content)
		outText = extractContentStr(choice.Message.Content)

		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				toolBlocks = append(toolBlocks, map[string]interface{}{
					"type":      "tool_call",
					"call_id":   tc.ID,
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}
		}

		finishReason := strings.ToLower(strings.TrimSpace(choice.FinishReason))
		if finishReason == "content_filter" {
			refusalBlock = map[string]interface{}{"type": "refusal", "reason": "content_filter"}
		}
	}

	msg := responsesMessage{
		Type:    "message",
		Role:    "assistant",
		Content: []map[string]interface{}{},
	}
	if strings.TrimSpace(outText) != "" {
		msg.Content = append(msg.Content, map[string]interface{}{"type": "output_text", "text": outText})
	}
	if len(toolBlocks) > 0 {
		msg.Content = append(msg.Content, toolBlocks...)
	}
	if refusalBlock != nil {
		msg.Content = append(msg.Content, refusalBlock)
	}

	rrsp := responsesResponse{
		ID:         resp.ID,
		Object:     "response",
		Created:    time.Now().Unix(),
		Model:      creq.Model,
		Output:     []responsesMessage{msg},
		OutputText: outText,
	}

	if resp.Usage.TotalTokens > 0 || resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		rrsp.Usage = &responsesUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return rrsp
}
