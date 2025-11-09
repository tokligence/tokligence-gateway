package tool_adapter

import (
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestAdaptTools_FilterApplyPatch(t *testing.T) {
	adapter := NewAdapter()

	tools := []openai.Tool{
		{
			Type: "function",
			Function: openai.ToolFunction{
				Name:        "apply_patch",
				Description: "Apply a patch to a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file": map[string]string{"type": "string"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{
				Name:        "read_file",
				Description: "Read file contents",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]string{"type": "string"},
					},
				},
			},
		},
	}

	result := adapter.AdaptTools(tools, "openai", "anthropic")

	// Should filter out apply_patch
	if !result.Modified {
		t.Error("Expected Modified to be true")
	}

	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool after filtering, got %d", len(result.Tools))
	}

	if result.Tools[0].Function.Name != "read_file" {
		t.Errorf("Expected read_file to remain, got %s", result.Tools[0].Function.Name)
	}

	if len(result.FilteredToolNames) != 1 || result.FilteredToolNames[0] != "apply_patch" {
		t.Errorf("Expected apply_patch in filtered names, got %v", result.FilteredToolNames)
	}
}

func TestAdaptTools_FilterUpdatePlan(t *testing.T) {
	adapter := NewAdapter()

	tools := []openai.Tool{
		{
			Type: "function",
			Function: openai.ToolFunction{
				Name:        "update_plan",
				Description: "Update execution plan",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{
				Name:        "shell",
				Description: "Execute shell command",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		},
	}

	result := adapter.AdaptTools(tools, "openai", "anthropic")

	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool after filtering, got %d", len(result.Tools))
	}

	if result.Tools[0].Function.Name != "shell" {
		t.Errorf("Expected shell to remain, got %s", result.Tools[0].Function.Name)
	}

	if len(result.FilteredToolNames) != 1 || result.FilteredToolNames[0] != "update_plan" {
		t.Errorf("Expected update_plan in filtered names, got %v", result.FilteredToolNames)
	}
}

func TestAdaptTools_MixedSupportedUnsupported(t *testing.T) {
	adapter := NewAdapter()

	tools := []openai.Tool{
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "apply_patch", Description: "Apply patch"},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "update_plan", Description: "Update plan"},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "shell", Description: "Shell command"},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "read_file", Description: "Read file"},
		},
	}

	result := adapter.AdaptTools(tools, "openai", "anthropic")

	if len(result.Tools) != 2 {
		t.Errorf("Expected 2 tools after filtering, got %d", len(result.Tools))
	}

	if len(result.FilteredToolNames) != 2 {
		t.Errorf("Expected 2 filtered tools, got %d", len(result.FilteredToolNames))
	}

	// Check that shell and read_file remain
	remainingNames := make(map[string]bool)
	for _, tool := range result.Tools {
		remainingNames[tool.Function.Name] = true
	}

	if !remainingNames["shell"] || !remainingNames["read_file"] {
		t.Error("Expected shell and read_file to remain")
	}
}

func TestAdaptTools_NoFilteringNeeded(t *testing.T) {
	adapter := NewAdapter()

	tools := []openai.Tool{
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "shell", Description: "Shell"},
		},
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "read_file", Description: "Read"},
		},
	}

	result := adapter.AdaptTools(tools, "openai", "anthropic")

	if result.Modified {
		t.Error("Expected Modified to be false when no filtering needed")
	}

	if len(result.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result.Tools))
	}

	if len(result.FilteredToolNames) != 0 {
		t.Errorf("Expected 0 filtered tools, got %d", len(result.FilteredToolNames))
	}
}

func TestAdaptTools_UnknownTranslationPair(t *testing.T) {
	adapter := NewAdapter()

	tools := []openai.Tool{
		{
			Type: "function",
			Function: openai.ToolFunction{Name: "apply_patch", Description: "Patch"},
		},
	}

	// Unknown translation pair should not filter
	result := adapter.AdaptTools(tools, "unknown", "unknown")

	if result.Modified {
		t.Error("Expected Modified to be false for unknown translation pair")
	}

	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool (no filtering), got %d", len(result.Tools))
	}
}

func TestAddGuidanceToMessages(t *testing.T) {
	adapter := NewAdapter()

	messages := []openai.ChatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
	}

	filteredTools := []string{"apply_patch"}

	result := adapter.AddGuidanceToMessages(messages, filteredTools, "openai", "anthropic")

	if len(result) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(result))
	}

	// First message should have guidance prepended
	if !contains(result[0].Content, "apply_patch") {
		t.Error("Expected guidance about apply_patch in system message")
	}

	if !contains(result[0].Content, "shell") {
		t.Error("Expected guidance to mention shell alternative")
	}
}

func TestAddGuidanceToMessages_NoSystemMessage(t *testing.T) {
	adapter := NewAdapter()

	messages := []openai.ChatMessage{
		{Role: "user", Content: "Hello"},
	}

	filteredTools := []string{"apply_patch"}

	result := adapter.AddGuidanceToMessages(messages, filteredTools, "openai", "anthropic")

	// Should prepend new system message
	if len(result) != 2 {
		t.Errorf("Expected 2 messages (new system + user), got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", result[0].Role)
	}
}

func TestAdaptToolChoice_FilteredToolReferenced(t *testing.T) {
	adapter := NewAdapter()

	// ToolChoice references a filtered tool
	toolChoice := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name": "apply_patch",
		},
	}

	filteredNames := []string{"apply_patch"}
	remainingTools := []openai.Tool{}

	result := adapter.AdaptToolChoice(toolChoice, filteredNames, remainingTools)

	// Should convert to "auto" since tool was filtered
	if result != "auto" {
		t.Errorf("Expected toolChoice to be converted to 'auto', got %v", result)
	}
}

func TestAdaptToolChoice_ValidTool(t *testing.T) {
	adapter := NewAdapter()

	toolChoice := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name": "shell",
		},
	}

	filteredNames := []string{"apply_patch"}
	remainingTools := []openai.Tool{
		{Function: openai.ToolFunction{Name: "shell"}},
	}

	result := adapter.AdaptToolChoice(toolChoice, filteredNames, remainingTools)

	// Should keep original toolChoice since shell is not filtered
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Error("Expected toolChoice to remain as map")
	}

	if resultMap["type"] != "function" {
		t.Error("Expected toolChoice type to remain 'function'")
	}
}

func TestAdaptChatRequest(t *testing.T) {
	adapter := NewAdapter()

	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "Use apply_patch for file changes."},
			{Role: "user", Content: "Fix the bug"},
		},
		Tools: []openai.Tool{
			{Function: openai.ToolFunction{Name: "apply_patch"}},
			{Function: openai.ToolFunction{Name: "shell"}},
		},
		ToolChoice: map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": "apply_patch",
			},
		},
	}

	result := adapter.AdaptChatRequest(req, "openai", "anthropic")

	// Should filter apply_patch tool
	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(result.Tools))
	}

	if result.Tools[0].Function.Name != "shell" {
		t.Error("Expected shell to remain")
	}

	// Should add guidance to system message
	if !contains(result.Messages[0].Content, "apply_patch") {
		t.Error("Expected guidance in system message")
	}

	// Should convert toolChoice to "auto"
	if result.ToolChoice != "auto" {
		t.Errorf("Expected toolChoice to be 'auto', got %v", result.ToolChoice)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[:len(substr)] == substr || anyIndexOf(s, substr) >= 0)
}

func anyIndexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
