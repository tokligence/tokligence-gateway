package anthropic_openai

import (
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "encoding/json"
)

// StripSystemReminder removes <system-reminder> ... </system-reminder> blocks from text.
func StripSystemReminder(s string) string {
    const startTag = "<system-reminder>"
    const endTag = "</system-reminder>"
    for {
        i := strings.Index(strings.ToLower(s), strings.ToLower(startTag))
        if i == -1 { break }
        j := strings.Index(strings.ToLower(s[i+len(startTag):]), strings.ToLower(endTag))
        if j == -1 {
            s = s[:i]
            break
        }
        j = i + len(startTag) + j
        s = s[:i] + s[j+len(endTag):]
    }
    return s
}

// FlattenBlocksText concatenates nested text blocks.
func FlattenBlocksText(blocks []ContentBlock) string {
    var b strings.Builder
    for _, c := range blocks {
        if strings.EqualFold(c.Type, "text") {
            b.WriteString(StripSystemReminder(c.Text))
        } else if len(c.Content) > 0 {
            b.WriteString(FlattenBlocksText(c.Content))
        }
    }
    return b.String()
}

// HasToolBlocks reports whether any message contains tool_use/tool_result blocks.
func HasToolBlocks(req Request) bool {
    for _, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            t := strings.ToLower(b.Type)
            if t == "tool_use" || t == "tool_result" { return true }
        }
    }
    return false
}

// HasToolResultBlocks reports whether any message contains tool_result blocks.
func HasToolResultBlocks(req Request) bool {
    for _, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_result") { return true }
        }
    }
    return false
}

// DetectReadOnlyIntent returns true when latest user text looks like read/summarize intent.
func DetectReadOnlyIntent(req Request) bool {
    for i := len(req.Messages) - 1; i >= 0; i-- {
        m := req.Messages[i]
        if strings.ToLower(m.Role) != "user" { continue }
        txt := strings.ToLower(FlattenBlocksText(m.Content.Blocks))
        if txt == "" { continue }
        roCues := []string{"readme", "read", "summary", "summarize", "summarise", "explain", "解释", "总结", "读取", "看看", "说明"}
        writeCues := []string{"edit", "update", "modify", "rewrite", "change", "apply_patch", "写", "修改", "更新", "重写"}
        hasRO := false
        for _, k := range roCues { if strings.Contains(txt, k) { hasRO = true; break } }
        if !hasRO { return false }
        for _, k := range writeCues { if strings.Contains(txt, k) { return false } }
        return true
    }
    return false
}

// IsReadOnlyTool returns whether a tool is safe when intent is read-only.
func IsReadOnlyTool(name string) bool {
    n := strings.ToLower(strings.TrimSpace(name))
    switch n {
    case "glob", "read", "grep", "ls", "notebookread", "webfetch", "websearch":
        return true
    default:
        return false
    }
}

// SuggestToolChoice suggests an OpenAI tool_choice value ("required", "auto", "none")
// based on the current Anthropic-style request context.
// Policy:
//  - If no tools declared: return "" (omit tool_choice)
//  - If there have been repeated Edit failures (3+): "none" to break the loop
//  - If Edit tool used 5+ times recently: "none" to prevent excessive attempts
//  - If this turn has tool_result referencing discovery tools (glob/grep/ls/websearch/webfetch): "required"
//  - If this turn has tool_result referencing read tools (read/notebookread): "none"
//  - If this turn has other tool_result: "auto"
//  - If no tool_result in this turn: "required" to encourage action continuity
func SuggestToolChoice(req Request) string {
    if len(req.Tools) == 0 { return "" }

    // Check for repeated Edit failures first to break out of error loops
    // Allow up to 5 failures before giving up
    if HasRecentEditFailures(req, 5) {
        return "none"
    }

    // Check if Edit tool has been used excessively (7+ times in recent 10 messages)
    if CountRecentToolUse(req, "edit", 10) >= 7 {
        return "none"
    }

    // Map tool_use id -> name and check for errors
    idToName := map[string]string{}
    idHasError := map[string]bool{}
    for _, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_use") {
                idToName[b.ID] = strings.ToLower(strings.TrimSpace(b.Name))
            }
            if strings.EqualFold(b.Type, "tool_result") && b.IsError {
                idHasError[b.ToolUseID] = true
            }
        }
    }

    // Restrict search for tool_result to messages AFTER the last assistant turn.
    // This approximates the current user turn rather than the entire history.
    lastAssistant := -1
    for i := len(req.Messages)-1; i >= 0; i-- {
        if strings.EqualFold(req.Messages[i].Role, "assistant") { lastAssistant = i; break }
    }

    lastName := ""
    lastToolHadError := false
    for i := len(req.Messages)-1; i > lastAssistant; i-- {
        blocks := req.Messages[i].Content.Blocks
        for j := len(blocks)-1; j >= 0; j-- {
            b := blocks[j]
            if strings.EqualFold(b.Type, "tool_result") {
                if b.ToolUseID != "" {
                    lastName = idToName[b.ToolUseID]
                    lastToolHadError = idHasError[b.ToolUseID]
                }
                goto decide
            }
        }
    }
decide:
    if lastName != "" {
        // If the last tool had an error and it's an Edit tool, use "none"
        if lastToolHadError && strings.EqualFold(lastName, "edit") {
            return "none"
        }

        switch lastName {
        case "glob", "grep", "ls", "websearch", "webfetch":
            return "required"
        case "read", "notebookread":
            // If the same user turn includes an edit/append/write intent, encourage action
            if DetectWriteIntent(req) { return "required" }
            return "none"
        default:
            return "auto"
        }
    }
    // No tool_result in this turn: encourage first action
    return "required"
}

// DetectWriteIntent returns true if the latest user message (after last assistant)
// includes cues that imply editing/appending/writing files.
func DetectWriteIntent(req Request) bool {
    lastAssistant := -1
    for i := len(req.Messages)-1; i >= 0; i-- {
        if strings.EqualFold(req.Messages[i].Role, "assistant") { lastAssistant = i; break }
    }
    for i := len(req.Messages)-1; i > lastAssistant; i-- {
        m := req.Messages[i]
        if !strings.EqualFold(m.Role, "user") { continue }
        txt := strings.ToLower(FlattenBlocksText(m.Content.Blocks))
        if txt == "" { continue }
        cues := []string{"edit", "update", "append", "add", "write", "apply_patch", "追加", "编辑", "修改", "增加", "添加"}
        for _, k := range cues { if strings.Contains(txt, k) { return true } }
        return false
    }
    return false
}

// ComputeWorkDir infers a preferred working directory from recent tool_use inputs
// (e.g., Read/Write/Edit) that contain file_path.
func ComputeWorkDir(req Request) string {
    for i := len(req.Messages)-1; i >= 0; i-- {
        m := req.Messages[i]
        for j := len(m.Content.Blocks)-1; j >= 0; j-- {
            b := m.Content.Blocks[j]
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            name := strings.ToLower(strings.TrimSpace(b.Name))
            switch name {
            case "read", "write", "edit", "multiedit":
                // Attempt to extract file_path from input (common pattern)
                if mp, ok := b.Input.(map[string]any); ok {
                    if v, ok := mp["file_path"].(string); ok && strings.TrimSpace(v) != "" {
                        dir := filepath.Dir(v)
                        if dir == "." || dir == "" { continue }
                        return dir
                    }
                }
            }
        }
    }
    return ""
}

// RewriteToolInputPaths adjusts file paths in tool inputs to avoid sandbox prefixes and
// to resolve relative paths relative to workdir when provided.
func RewriteToolInputPaths(toolName string, input any, workdir string) any {
    mp, ok := input.(map[string]any)
    if !ok { return input }
    fixPath := func(p string) string {
        if p == "" { return p }
        if strings.HasPrefix(p, "/sandbox/") {
            p = strings.TrimPrefix(p, "/sandbox/")
            if workdir != "" { p = filepath.Join(workdir, p) }
            return p
        }
        // If not absolute and workdir provided, join
        if !strings.HasPrefix(p, "/") && workdir != "" {
            return filepath.Join(workdir, p)
        }
        return p
    }
    if v, ok := mp["file_path"].(string); ok {
        mp["file_path"] = fixPath(v)
    }
    if arr, ok := mp["files"].([]any); ok {
        for i := range arr {
            if fm, ok := arr[i].(map[string]any); ok {
                if pv, ok := fm["path"].(string); ok {
                    fm["path"] = fixPath(pv)
                }
            }
        }
    }
    return mp
}

// IsDuplicateToolUse returns true if the same tool with identical input
// appears recently in the request history (after the last assistant turn).
func IsDuplicateToolUse(req Request, name string, input any) bool {
    key := canonicalKey(input)
    lastAssistant := -1
    for i := len(req.Messages)-1; i >= 0; i-- {
        if strings.EqualFold(req.Messages[i].Role, "assistant") { lastAssistant = i; break }
    }
    for i := len(req.Messages)-1; i > lastAssistant; i-- {
        m := req.Messages[i]
        for j := len(m.Content.Blocks)-1; j >= 0; j-- {
            b := m.Content.Blocks[j]
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !strings.EqualFold(b.Name, name) { continue }
            if canonicalKey(b.Input) == key { return true }
            // For Edit tool, also check for similar patterns (same file, similar old_string)
            if strings.EqualFold(name, "edit") && IsSimilarEditAttempt(b.Input, input) {
                return true
            }
        }
    }
    return false
}

// IsSimilarEditAttempt checks if two Edit tool inputs target the same file
// with similar but not identical old_string patterns (common in failed edit loops)
func IsSimilarEditAttempt(prev, current any) bool {
    prevMap, ok1 := prev.(map[string]any)
    currMap, ok2 := current.(map[string]any)
    if !ok1 || !ok2 {
        return false
    }

    // Must target the same file
    prevFile, _ := prevMap["file_path"].(string)
    currFile, _ := currMap["file_path"].(string)
    if prevFile == "" || currFile == "" || prevFile != currFile {
        return false
    }

    // Check if old_string patterns are similar (common variations in failed edits)
    prevOld, _ := prevMap["old_string"].(string)
    currOld, _ := currMap["old_string"].(string)

    // Common patterns in failed edit loops:
    // 1. Same base content with different line numbers (e.g., "\n\n" vs "\n443→\n")
    // 2. Same content with/without quotes
    // 3. Same content with different whitespace
    if prevOld == currOld {
        return true
    }

    // Check if both contain mostly newlines (common failed pattern)
    prevNewlines := strings.Count(prevOld, "\n")
    currNewlines := strings.Count(currOld, "\n")
    if prevNewlines > 0 && currNewlines > 0 {
        // If both are mostly newlines, consider them similar
        prevNonNewline := strings.ReplaceAll(prevOld, "\n", "")
        currNonNewline := strings.ReplaceAll(currOld, "\n", "")
        // If after removing newlines, the remaining content is similar or one contains line numbers
        if len(prevNonNewline) < 10 && len(currNonNewline) < 10 {
            return true
        }
        // Check if one contains line numbers (e.g., "443→")
        if strings.Contains(currOld, "→") || strings.Contains(prevOld, "→") {
            return true
        }
    }

    return false
}

// canonicalKey produces a stable, deterministic string key for arbitrary JSON-like values
// by sorting map keys and rendering primitives in a normalized format.
func canonicalKey(v any) string {
    switch x := v.(type) {
    case nil:
        return "null"
    case string:
        b, _ := json.Marshal(x)
        return string(b)
    case bool:
        if x { return "true" }
        return "false"
    case float64:
        return strconv.FormatFloat(x, 'g', -1, 64)
    case int:
        return strconv.Itoa(x)
    case int64:
        return strconv.FormatInt(x, 10)
    case json.Number:
        return x.String()
    case map[string]any:
        if len(x) == 0 { return "{}" }
        keys := make([]string, 0, len(x))
        for k := range x { keys = append(keys, k) }
        sort.Strings(keys)
        var b strings.Builder
        b.WriteByte('{')
        for i, k := range keys {
            if i > 0 { b.WriteByte(',') }
            kb, _ := json.Marshal(k)
            b.Write(kb)
            b.WriteByte(':')
            b.WriteString(canonicalKey(x[k]))
        }
        b.WriteByte('}')
        return b.String()
    case []any:
        if len(x) == 0 { return "[]" }
        var b strings.Builder
        b.WriteByte('[')
        for i, e := range x {
            if i > 0 { b.WriteByte(',') }
            b.WriteString(canonicalKey(e))
        }
        b.WriteByte(']')
        return b.String()
    default:
        bb, _ := json.Marshal(x)
        return string(bb)
    }
}

// ExtractFilePath tries to read a file_path-like field from tool input.
func ExtractFilePath(input any) string {
    if mp, ok := input.(map[string]any); ok {
        if v, ok := mp["file_path"].(string); ok { return v }
        if v, ok := mp["path"].(string); ok { return v }
    }
    return ""
}

// ExtractSearchPattern reads a search/glob pattern from tool input when present.
func ExtractSearchPattern(input any) string {
    if mp, ok := input.(map[string]any); ok {
        if v, ok := mp["pattern"].(string); ok { return v }
    }
    return ""
}

// IsRecentSameTarget returns true if the same tool (by name) targeted the same
// primary identifier (file_path or pattern) within the recent window of messages.
func IsRecentSameTarget(req Request, name, target string, window int) bool {
    if strings.TrimSpace(target) == "" { return false }
    lcName := strings.ToLower(strings.TrimSpace(name))
    count := 0
    for i := len(req.Messages)-1; i >= 0 && count < window; i-- {
        count++
        m := req.Messages[i]
        for j := len(m.Content.Blocks)-1; j >= 0; j-- {
            b := m.Content.Blocks[j]
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !strings.EqualFold(b.Name, lcName) { continue }
            t := ExtractFilePath(b.Input)
            if t == "" && (lcName == "glob" || lcName == "grep" || lcName == "ls") {
                t = ExtractSearchPattern(b.Input)
            }
            if t != "" && t == target { return true }
        }
    }
    return false
}

// HasRecentEditFailures checks if there have been multiple failed Edit tool attempts
// on the same file within recent messages, indicating a problematic edit pattern.
func HasRecentEditFailures(req Request, threshold int) bool {
    editFailures := make(map[string]int) // file_path -> failure count

    // Look through recent messages for Edit tool failures
    for i := len(req.Messages)-1; i >= 0 && i >= len(req.Messages)-10; i-- {
        m := req.Messages[i]
        for _, b := range m.Content.Blocks {
            // Check for Edit tool_use
            if strings.EqualFold(b.Type, "tool_use") && strings.EqualFold(b.Name, "edit") {
                filePath := ExtractFilePath(b.Input)
                if filePath != "" {
                    // Look for corresponding tool_result with error
                    for j := i; j < len(req.Messages) && j <= i+1; j++ {
                        for _, rb := range req.Messages[j].Content.Blocks {
                            if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID && rb.IsError {
                                editFailures[filePath]++
                                break
                            }
                        }
                    }
                }
            }
        }
    }

    // Check if any file has exceeded the failure threshold
    for _, count := range editFailures {
        if count >= threshold {
            return true
        }
    }
    return false
}

// GetAllToolUseIDs collects all tool_use IDs from the request history
// This helps identify which tool calls have already been processed
func GetAllToolUseIDs(req Request) map[string]bool {
    ids := make(map[string]bool)
    for _, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_use") && b.ID != "" {
                ids[b.ID] = true
            }
        }
    }
    return ids
}

// HasSuccessfulToolResult checks if there's already a successful result for a specific tool and input
func HasSuccessfulToolResult(req Request, toolName string, input any) bool {
    // Look for matching tool_use with successful result (search forward across messages)
    for i, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_use") && strings.EqualFold(b.Name, toolName) {
                // Check if inputs match
                if canonicalKey(b.Input) == canonicalKey(input) {
                    // Look for corresponding successful tool_result anywhere after this tool_use
                    for j := i; j < len(req.Messages); j++ {
                        for _, rb := range req.Messages[j].Content.Blocks {
                            if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID && !rb.IsError {
                                return true
                            }
                        }
                    }
                }
            }
        }
    }
    return false
}

// HasAnyToolResult checks if there's already a tool_result for a specific tool and input,
// regardless of error status. Useful to suppress repeated discovery calls that already yielded
// a result (even if empty or error) earlier in the conversation history.
func HasAnyToolResult(req Request, toolName string, input any) bool {
    for i, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_use") && strings.EqualFold(b.Name, toolName) {
                if canonicalKey(b.Input) == canonicalKey(input) {
                    // Look for any corresponding tool_result (error or not) anywhere after this
                    for j := i; j < len(req.Messages); j++ {
                        for _, rb := range req.Messages[j].Content.Blocks {
                            if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID {
                                return true
                            }
                        }
                    }
                }
            }
        }
    }
    return false
}

// IsDiscoveryTool returns true for read-only, discovery-oriented tools where repeating the
// same input yields little value and can safely be suppressed when a result already exists.
func IsDiscoveryTool(name string) bool {
    n := strings.ToLower(strings.TrimSpace(name))
    switch n {
    case "glob", "grep", "ls", "search", "websearch", "webfetch":
        return true
    default:
        return false
    }
}

// HasRecentSameInput returns true if an identical tool_use (same name and identical input)
// appears within the last 'window' messages, regardless of assistant/user boundaries.
func HasRecentSameInput(req Request, name string, input any, window int) bool {
    key := canonicalKey(input)
    seen := 0
    for i := len(req.Messages)-1; i >= 0 && seen < window; i-- {
        seen++
        m := req.Messages[i]
        for j := len(m.Content.Blocks)-1; j >= 0; j-- {
            b := m.Content.Blocks[j]
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !strings.EqualFold(b.Name, name) { continue }
            if canonicalKey(b.Input) == key { return true }
        }
    }
    return false
}

// CountRecentToolUse counts how many times a specific tool has been used recently
func CountRecentToolUse(req Request, toolName string, window int) int {
    count := 0
    seen := 0
    lcName := strings.ToLower(strings.TrimSpace(toolName))

    for i := len(req.Messages)-1; i >= 0 && seen < window; i-- {
        m := req.Messages[i]
        seen++
        for _, b := range m.Content.Blocks {
            if strings.EqualFold(b.Type, "tool_use") && strings.EqualFold(b.Name, lcName) {
                count++
            }
        }
    }
    return count
}

// IsWriteLikeTool returns true for tools that modify file contents.
func IsWriteLikeTool(name string) bool {
    n := strings.ToLower(strings.TrimSpace(name))
    switch n {
    case "edit", "update", "write", "append":
        return true
    default:
        return false
    }
}

// ExtractNewContent tries to retrieve the new content provided to a write-like tool.
// For edit/update tools, it looks for new_string; for write-like tools, it also checks content/text fields.
func ExtractNewContent(input any) string {
    if mp, ok := input.(map[string]any); ok {
        if v, ok := mp["new_string"].(string); ok { return v }
        if v, ok := mp["content"].(string); ok { return v }
        if v, ok := mp["text"].(string); ok { return v }
    }
    return ""
}

// HasSuccessfulWriteOnFileWithContent checks if there is a prior successful write-like operation
// on the same file producing identical new content.
func HasSuccessfulWriteOnFileWithContent(req Request, filePath, newContent string) bool {
    if strings.TrimSpace(filePath) == "" || strings.TrimSpace(newContent) == "" { return false }
    for i, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !IsWriteLikeTool(b.Name) { continue }
            if ExtractFilePath(b.Input) != filePath { continue }
            if ExtractNewContent(b.Input) != newContent { continue }
            // verify corresponding successful tool_result searching forward
            for j := i; j < len(req.Messages); j++ {
                for _, rb := range req.Messages[j].Content.Blocks {
                    if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID && !rb.IsError {
                        return true
                    }
                }
            }
        }
    }
    return false
}

// HasRecentChangeOnFile checks if there has been a recent successful write-like operation
// on the specified file within a sliding window of messages.
func HasRecentChangeOnFile(req Request, filePath string, window int) bool {
    if strings.TrimSpace(filePath) == "" { return false }
    seen := 0
    for i := len(req.Messages)-1; i >= 0 && seen < window; i-- {
        seen++
        m := req.Messages[i]
        for _, b := range m.Content.Blocks {
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !IsWriteLikeTool(b.Name) { continue }
            if ExtractFilePath(b.Input) != filePath { continue }
            // Look for success result forward
            for j := i; j < len(req.Messages); j++ {
                for _, rb := range req.Messages[j].Content.Blocks {
                    if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID && !rb.IsError {
                        return true
                    }
                }
            }
        }
    }
    return false
}

// HasRecentWriteSuccessOnFile checks if there's a successful write-like tool_result
// on the same file within the last 'window' messages.
func HasRecentWriteSuccessOnFile(req Request, filePath string, window int) bool {
    if strings.TrimSpace(filePath) == "" { return false }
    seen := 0
    for i := len(req.Messages)-1; i >= 0 && seen < window; i-- {
        seen++
        m := req.Messages[i]
        for _, b := range m.Content.Blocks {
            if !strings.EqualFold(b.Type, "tool_use") { continue }
            if !IsWriteLikeTool(b.Name) { continue }
            if ExtractFilePath(b.Input) != filePath { continue }
            // success result forward
            for j := i; j < len(req.Messages); j++ {
                for _, rb := range req.Messages[j].Content.Blocks {
                    if strings.EqualFold(rb.Type, "tool_result") && rb.ToolUseID == b.ID && !rb.IsError {
                        return true
                    }
                }
            }
        }
    }
    return false
}
