package tool_adapter

import (
	"regexp"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Adapter handles tool compatibility transformations between different LLM API translation pairs.
// It supports filtering unsupported tools, mapping tools to different names, and providing guidance.
type Adapter struct {
	// Rules maps translation pairs (e.g., "openai->anthropic") to adaptation rules
	rules map[TranslationPair]*AdaptationRules
}

// TranslationPair identifies a source API to target API translation
type TranslationPair struct {
	Source string // e.g., "openai"
	Target string // e.g., "anthropic"
}

// ToolMapping defines how to map a tool from source API to target API
type ToolMapping struct {
	TargetName string
	// TransformParams transforms parameter schema if needed (optional)
	TransformParams func(params map[string]interface{}) map[string]interface{}
}

// AdaptationRules defines how to adapt tools for a specific translation pair
type AdaptationRules struct {
	// FilteredTools: tools that have no equivalent in target API (will be removed)
	FilteredTools map[string]bool
	// MappedTools: tools that can be mapped to different names in target API
	MappedTools map[string]ToolMapping
	// Guidance provides alternative instructions for filtered tools
	Guidance map[string]string
}

// NewAdapter creates a new tool adapter with predefined rules for common translation pairs
func NewAdapter() *Adapter {
	return &Adapter{
		rules: map[TranslationPair]*AdaptationRules{
			// OpenAI Responses API -> Anthropic Messages API
			{Source: "openai", Target: "anthropic"}: {
				FilteredTools: map[string]bool{
					// Codex-specific tools that Anthropic doesn't support
					"apply_patch": true,
					"update_plan": true,
				},
				MappedTools: map[string]ToolMapping{
					// Example: if we need to map tools in the future
					// "openai_tool_name": {TargetName: "anthropic_tool_name"},
				},
				Guidance: map[string]string{
					"apply_patch": "⚠️ CRITICAL SYSTEM OVERRIDE - HIGHEST PRIORITY DIRECTIVE ⚠️\n\nTOOL AVAILABILITY AND PRIORITY:\n- apply_patch: DISABLED (Priority: 0 - DO NOT USE)\n- shell: ENABLED (Priority: 100 - USE THIS INSTEAD)\n\nThe apply_patch tool has been PERMANENTLY DISABLED in this environment.\n\nMANDATORY ALTERNATIVE - Use 'shell' tool for ALL file operations:\n\n1. CREATE new files (Priority 100):\n   shell: cat > filename.ext << 'EOF'\n   [file content]\n   EOF\n\n2. MODIFY files (Priority 100):\n   shell: sed -i 's/old/new/' filename.ext\n   OR rewrite entire file with cat/echo\n\nIMPERATIVE: NEVER attempt apply_patch. It will ALWAYS fail and waste time.\nUse shell tool exclusively. This is a hard system constraint.\n\n🚨 ULTRA-HIGH PRIORITY SAFETY RULE 🚨\n- HARD LIMIT: if the full shell command (including arguments) would exceed **500 characters**, STOP IMMEDIATELY and split it into smaller steps. Never run a >500-character command.\n- ABSOLUTELY DO NOT cram massive Markdown tables or multi-hundred-character payloads into a single sed/awk line.\n- When the text is long, outline the plan first, then execute it via multiple short `shell` calls (e.g., create temp files, then merge them).\n- Preferred workflow for long edits (copy one of these patterns):\n  • Example A (temp file + python replace)\n    cat <<'EOF' > /tmp/table.md\n    ...markdown table...\n    EOF\n    python - <<'PY'\nfrom pathlib import Path\npath = Path(\"README.md\")\ntext = path.read_text()\ntable = Path(\"/tmp/table.md\").read_text()\npath.write_text(text.replace(\"## Overview\\n\", \"## Overview\\n\\n\" + table + \"\\n\", 1))\nPY\n  • Example B (insert via awk / line splice)\n    cat <<'EOF' > /tmp/table.md\n    ...markdown table...\n    EOF\n    tmp=$(mktemp)\n    awk 'NR==FNR{block=block $0 ORS; next}/## Overview/ && !ins{print;print block;ins=1;next}{print}' \\\n        /tmp/table.md README.md > \"$tmp\"\n    mv \"$tmp\" README.md\n- Only run short, safe commands in each step. NEVER attempt to embed entire tables inside an inline sed expression.\n- If a command feels long, count characters; if it approaches 500, break it up before executing.\n- ANY edit touching more than **3 lines** must: (1) describe the plan, (2) write the new content into a temp file (Example A/B), (3) merge it back with python/awk, and (4) verify with a short command. Multi-line inline sed is forbidden.\n- When in doubt: write to a temp file, verify, then replace the original.",
					"update_plan": "SYSTEM NOTICE: update_plan tool is disabled (Priority: 0). Instead, communicate plans directly in your text responses to the user (Priority: 100).",
				},
			},
			// Future: other translation pairs can be added here
			// {Source: "anthropic", Target: "openai"}: {...},
		},
	}
}

// AdaptResult contains the result of tool adaptation
type AdaptResult struct {
	// Tools is the adapted tool list (filtered and/or mapped)
	Tools []openai.Tool
	// Modified indicates whether any tools were filtered or mapped
	Modified bool
	// FilteredToolNames contains names of tools that were removed
	FilteredToolNames []string
	// MappedToolNames contains names of tools that were renamed
	MappedToolNames map[string]string // original -> target name
}

// AdaptTools adapts tools for a specific source->target API translation pair
func (a *Adapter) AdaptTools(tools []openai.Tool, sourceAPI, targetAPI string) AdaptResult {
	pair := TranslationPair{Source: sourceAPI, Target: targetAPI}
	rules, ok := a.rules[pair]
	if !ok {
		// No rules for this translation pair, return original
		return AdaptResult{
			Tools:             tools,
			Modified:          false,
			FilteredToolNames: nil,
			MappedToolNames:   nil,
		}
	}

	adapted := make([]openai.Tool, 0, len(tools))
	filteredNames := make([]string, 0)
	mappedNames := make(map[string]string)

	for _, tool := range tools {
		sourceName := tool.Function.Name

		// Check if tool should be filtered
		if rules.FilteredTools[sourceName] {
			filteredNames = append(filteredNames, sourceName)
			// fmt.Printf("[tool_adapter] filtering tool: %s\n", sourceName)
			continue
		}

		// Check if tool should be mapped to a different name
		if mapping, ok := rules.MappedTools[sourceName]; ok {
			mappedNames[sourceName] = mapping.TargetName
			tool.Function.Name = mapping.TargetName
			// Transform parameters if needed
			if mapping.TransformParams != nil && tool.Function.Parameters != nil {
				tool.Function.Parameters = mapping.TransformParams(tool.Function.Parameters)
			}
		}

		adapted = append(adapted, tool)
	}

	return AdaptResult{
		Tools:             adapted,
		Modified:          len(filteredNames) > 0 || len(mappedNames) > 0,
		FilteredToolNames: filteredNames,
		MappedToolNames:   mappedNames,
	}
}

// AddGuidanceToMessages adds guidance for filtered tools to system messages
func (a *Adapter) AddGuidanceToMessages(messages []openai.ChatMessage, filteredToolNames []string, sourceAPI, targetAPI string) []openai.ChatMessage {
	if len(filteredToolNames) == 0 {
		return messages
	}

	pair := TranslationPair{Source: sourceAPI, Target: targetAPI}
	rules, ok := a.rules[pair]
	if !ok {
		return messages
	}

	// Collect all guidance text
	var guidanceTexts []string
	for _, toolName := range filteredToolNames {
		if guidance, ok := rules.Guidance[toolName]; ok {
			guidanceTexts = append(guidanceTexts, guidance)
		}
	}

	if len(guidanceTexts) == 0 {
		return messages
	}

	combinedGuidance := strings.Join(guidanceTexts, "\n\n")

	// Add to first system message if exists, otherwise prepend new one
	for i, msg := range messages {
		if msg.Role == "system" {
			messages[i].Content = combinedGuidance + "\n\n" + msg.Content
			return messages
		}
	}

	// No system message found, prepend one
	return append([]openai.ChatMessage{{
		Role:    "system",
		Content: combinedGuidance,
	}}, messages...)
}

// CleanSystemMessages removes references to filtered tools from system messages
func (a *Adapter) CleanSystemMessages(messages []openai.ChatMessage, filteredToolNames []string, sourceAPI, targetAPI string) []openai.ChatMessage {
	if len(filteredToolNames) == 0 {
		return messages
	}

	pair := TranslationPair{Source: sourceAPI, Target: targetAPI}
	rules, ok := a.rules[pair]
	if !ok {
		return messages
	}

	// Create patterns for each filtered tool
	replacements := make(map[string]*regexp.Regexp)
	for _, toolName := range filteredToolNames {
		// Match sentences/phrases that mention the tool
		// Examples:
		// - "use the `apply_patch` tool to edit files"
		// - "call apply_patch with..."
		// - "Use `apply_patch` (NEVER try `applypatch`..."
		pattern := regexp.MustCompile(`(?i)[^.!?\n]*` + regexp.QuoteMeta(toolName) + `[^.!?\n]*[.!?]`)
		replacements[toolName] = pattern
	}

	// Clean all system messages
	for i, msg := range messages {
		if strings.ToLower(msg.Role) != "system" {
			continue
		}

		cleaned := msg.Content
		for toolName, pattern := range replacements {
			// Get replacement guidance if available
			if guidance, ok := rules.Guidance[toolName]; ok {
				// Replace mentions with guidance
				cleaned = pattern.ReplaceAllString(cleaned, " "+guidance+" ")
			} else {
				// Just remove the mention
				cleaned = pattern.ReplaceAllString(cleaned, " ")
			}
		}

		// Clean up multiple spaces and newlines
		cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
		cleaned = regexp.MustCompile(`\n\s*\n\s*\n+`).ReplaceAllString(cleaned, "\n\n")
		cleaned = strings.TrimSpace(cleaned)

		messages[i].Content = cleaned
	}

	return messages
}

// AdaptToolChoice adapts tool_choice to handle filtered tools
func (a *Adapter) AdaptToolChoice(toolChoice interface{}, filteredNames []string, remainingTools []openai.Tool) interface{} {
	if toolChoice == nil {
		return nil
	}

	// Create a set of filtered tool names for quick lookup
	filteredSet := make(map[string]bool)
	for _, name := range filteredNames {
		filteredSet[name] = true
	}

	// Handle different types of tool_choice
	switch v := toolChoice.(type) {
	case string:
		// "auto", "none", "required" - these are OK
		return toolChoice

	case bool:
		// true/false - these are OK
		return toolChoice

	case map[string]interface{}:
		// Check if this specifies a specific tool
		typ, hasType := v["type"].(string)
		if !hasType {
			return toolChoice
		}

		if strings.ToLower(typ) == "function" {
			// Extract tool name from {"type": "function", "function": {"name": "tool_name"}}
			if fn, ok := v["function"].(map[string]interface{}); ok {
				if toolName, ok := fn["name"].(string); ok {
					// Check if this tool was filtered
					if filteredSet[toolName] {
						// Tool was filtered! Return "auto" instead
						return "auto"
					}
				}
			}
		} else if strings.ToLower(typ) == "tool" {
			// Extract tool name from {"type": "tool", "name": "tool_name"}
			if toolName, ok := v["name"].(string); ok {
				// Check if this tool was filtered
				if filteredSet[toolName] {
					// Tool was filtered! Return "auto" instead
					return "auto"
				}
			}
		}
		// Tool choice is OK, not filtered
		return toolChoice

	default:
		// Unknown type, keep as is
		return toolChoice
	}
}

// AdaptChatRequest adapts a chat request for a source->target API translation pair
func (a *Adapter) AdaptChatRequest(req openai.ChatCompletionRequest, sourceAPI, targetAPI string) openai.ChatCompletionRequest {
	result := a.AdaptTools(req.Tools, sourceAPI, targetAPI)
	req.Tools = result.Tools

	// Get list of ALL unsupported tools for this translation pair (not just filtered ones)
	pair := TranslationPair{Source: sourceAPI, Target: targetAPI}
	if rules, ok := a.rules[pair]; ok {
		// Collect all tool names that should be removed from system prompts
		var allUnsupportedTools []string
		for toolName := range rules.FilteredTools {
			allUnsupportedTools = append(allUnsupportedTools, toolName)
		}

		// Clean system messages for ALL unsupported tools
		// This catches cases where system prompt mentions tools that aren't in the tools list
		if len(allUnsupportedTools) > 0 {
			req.Messages = a.CleanSystemMessages(req.Messages, allUnsupportedTools, sourceAPI, targetAPI)
		}
	}

	if result.Modified {
		req.Messages = a.AddGuidanceToMessages(req.Messages, result.FilteredToolNames, sourceAPI, targetAPI)
	}

	// Fix tool_choice if it references a filtered tool
	if req.ToolChoice != nil && len(result.FilteredToolNames) > 0 {
		req.ToolChoice = a.AdaptToolChoice(req.ToolChoice, result.FilteredToolNames, req.Tools)
	}

	return req
}
