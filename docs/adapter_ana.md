# Claude Code to OpenAI API Bridge Analysis

## Problem Statement

When using Claude Code (Anthropic's CLI) through a gateway that bridges to OpenAI's API, we encountered issues with duplicate tool calls being displayed and tool execution failures. This document analyzes the root causes and explores potential solutions.

## The Original Issues

### 1. Duplicate Tool Display
Claude Code was showing the same tool calls multiple times:
```
● Search(pattern: "README")
  ⎿  Found 21 files (ctrl+o to expand)

● Search(pattern: "README")
  ⎿  Found 21 files (ctrl+o to expand)

● Search(pattern: "README.md")
  ⎿  Found 14 files (ctrl+o to expand)
```

### 2. Tool Execution Failures
The Edit/Update tool was failing repeatedly due to ambiguous string matching, causing infinite retry loops.

## Why Session Management Was the Wrong Solution

### Initial Hypothesis
We initially thought the problem was that Claude Code sends accumulated message history with each request, and we needed to deduplicate tools across multiple API calls within the same conversation.

### The Session Management Approach
We implemented a session management system that:
1. Created sessions based on user ID and message content
2. Tracked processed tools across multiple requests
3. Filtered out "duplicate" tools in subsequent requests

### Why It Failed

#### 1. Fundamental Misunderstanding of Claude Code's Behavior
Claude Code operates in a multi-request paradigm:
- Each user command triggers multiple API requests
- Each request is a step in the workflow (search → read → edit)
- These are NOT duplicates - they're sequential operations

Example workflow for "Add line to README":
1. Request 1: Search for README files
2. Request 2: Read the README content
3. Request 3: Edit the README with new content
4. Request 4: Verify the change

#### 2. Session Key Generation Problems
Our session identification was flawed:
```go
// Original: Used message prefix (first 100 chars)
msgPrefix := userMessage[:100]
sessionKey := fmt.Sprintf("%s:%s", userID, msgPrefix)

// Problem: Similar messages created same session
"Add line to README: test 2.8"
"Add line to README: test 2.9"  // Same first 100 chars!
```

Even when we switched to MD5 hashing:
```go
msgHash := md5.Sum([]byte(userMessage))
sessionKey := fmt.Sprintf("%s:%s", userID, msgHashStr)

// Problem: Same message in different requests = same session
// But these are different steps in the workflow!
```

#### 3. Wrong Granularity
Session management tried to maintain state across requests, but:
- Each API request should be independent
- Tool deduplication should only happen WITHIN a single request
- Cross-request state prevents normal workflow execution

#### 4. The Real Problem Was Elsewhere
The actual issues were:
- Claude Code accumulating history in each request (expected behavior)
- Our aggressive duplicate suppression within single requests
- Edit tool retry logic being too strict

## Root Cause Analysis

### Claude Code's Request Pattern
```
Request 1: User message + Assistant searches
Request 2: User message + Search results + Assistant reads file
Request 3: User message + Search + Read + Assistant edits
Request 4: User message + Search + Read + Edit + Assistant confirms
```

Each request contains the full conversation history up to that point. This is by design - it maintains context.

### The Gateway's Misinterpretation
Our gateway saw the repeated tool mentions and tried to "optimize" by:
1. Detecting "duplicate" tools across requests
2. Filtering them out
3. Returning empty responses or error messages

This broke Claude Code's workflow because it expected to see its tools processed.

## Why Session Management Cannot Work Here

### 1. Stateless API Design
REST APIs are meant to be stateless. Each request should contain all necessary information. Adding session state violates this principle and creates complexity.

### 2. No Unique Request Identifier
We tried to identify "same conversation" using:
- User ID (too broad - all requests from same user)
- Message content (too narrow - changes slightly)
- Claude session ID (exists but changes meaning across requests)

None of these accurately captured "this is a duplicate request" vs "this is the next step".

### 3. Tool Repetition Is Normal
In a coding workflow, it's normal to:
- Search multiple times with refined queries
- Read the same file multiple times
- Attempt edits multiple times with corrections

Session-based deduplication prevented these legitimate operations.

## Correct Solutions

### 1. Single-Request Deduplication Only
Only remove duplicates within a single API request's message history:

```python
def process_request(messages):
    seen_tools = set()
    for message in messages:
        for tool in message.tools:
            tool_key = (tool.name, hash(tool.input))
            if tool_key not in seen_tools:
                process_tool(tool)
                seen_tools.add(tool_key)
```

### 2. Smart History Truncation
Instead of deduplicating, intelligently truncate history:

```python
def truncate_history(messages, max_tools=10):
    # Keep only the most recent N tool uses
    # Preserve all tool results
    # Keep all text messages
    pass
```

### 3. Tool-Specific Retry Logic
Different tools need different retry strategies:

```python
retry_policies = {
    "Edit": {"max_attempts": 5, "backoff": "exponential"},
    "Read": {"max_attempts": 3, "backoff": "none"},
    "Search": {"max_attempts": 1, "backoff": "none"},  # Don't retry searches
}
```

### 4. Response Streaming Optimization
For streaming responses, send incremental updates instead of full history:

```python
def stream_response(full_response, previous_response):
    # Only send the delta between responses
    new_content = full_response[len(previous_response):]
    yield new_content
```

### 5. Context Window Management
Implement proper context window management:

```python
def manage_context(messages, max_tokens=100000):
    # Prioritize recent messages
    # Summarize old tool results
    # Keep essential context
    pass
```

## Future Improvements

### 1. Request Fingerprinting
Create better request identification:
```python
def fingerprint_request(request):
    # Combine multiple signals
    return hash((
        request.user_id,
        request.timestamp,
        request.last_tool_id,
        request.message_count
    ))
```

### 2. Workflow-Aware Processing
Understand common workflows and optimize for them:
```python
workflows = {
    "file_edit": ["search", "read", "edit", "verify"],
    "code_analysis": ["search", "read", "analyze"],
}
```

### 3. Client-Side Hints
Allow Claude Code to send hints about request intent:
```json
{
    "messages": [...],
    "metadata": {
        "request_intent": "retry_previous",
        "workflow_step": 3,
        "expect_tools": ["Edit"]
    }
}
```

### 4. Tool Result Caching
Cache tool results at the gateway level (not session-based):
```python
# Cache based on tool input, not request
cache_key = f"{tool.name}:{hash(tool.input)}"
if cache_key in tool_cache:
    return tool_cache[cache_key]
```

## Lessons Learned

1. **Don't Fight the Protocol**: Claude Code's multi-request pattern is intentional. Work with it, not against it.

2. **Stateless is Better**: Adding state to a stateless protocol creates more problems than it solves.

3. **Understand the Full Flow**: Before optimizing, understand the complete workflow from client to server and back.

4. **Test with Real Workflows**: Unit tests aren't enough. Test with actual Claude Code workflows to catch integration issues.

5. **Simple Solutions First**: The simplest solution (disabling deduplication) was the correct one.

## Conclusion

Session management was a well-intentioned but fundamentally flawed approach to solving the duplicate tool problem. The issue wasn't that tools were being duplicated across requests - it was that we were too aggressive in deduplicating within requests.

The correct approach is to:
1. Accept that Claude Code sends multiple requests per conversation
2. Process each request independently
3. Only deduplicate within single requests
4. Optimize for the common workflows
5. Keep the gateway stateless

By removing session management and focusing on single-request optimization, we achieve a simpler, more reliable, and more maintainable solution.