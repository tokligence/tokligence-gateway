# Gateway Test Suite

Organized test suite for Tokligence Gateway functionality.

## Directory Structure

```
tests/
├── integration/              # Integration tests
│   ├── tool_calls/          # Tool calling tests
│   ├── responses_api/       # OpenAI Responses API tests
│   ├── streaming/           # Streaming SSE tests
│   └── duplicate_detection/ # Duplicate tool call detection
├── fixtures/                # Test data and request samples
│   ├── tool_calls/          # Tool call request samples
│   └── tool_adapter/        # Tool adapter test cases
├── utils/                   # Utility scripts
├── config/                  # Configuration tests
└── run_all_tests.sh         # Master test runner
```

## Quick Start

**Run all tests:**
```bash
./run_all_tests.sh
```

**Run specific test category:**
```bash
# Tool calling tests
./integration/tool_calls/test_basic.sh
./integration/tool_calls/test_full_flow.sh

# Responses API tests
./integration/responses_api/test_conversion.sh

# Streaming tests
./integration/streaming/test_sse_events.sh

# Duplicate detection tests
./integration/duplicate_detection/test_detection.sh
```

**Run configuration tests:**
```bash
./config/test_multiport.sh
```

## Test Categories

### Integration Tests (`integration/`)

#### Tool Calls (`integration/tool_calls/`)
- `test_basic.sh` - Basic tool calling functionality
- `test_full_flow.sh` - Complete request→tool→response flow
- `test_multiple.sh` - Multiple tool calls in one request
- `test_nonstreaming.sh` - Non-streaming tool calls
- `test_validation.sh` - Tool call format validation
- `test_math.sh` - Math tool calling test
- `test_hang.sh` - Hang detection test

#### Responses API (`integration/responses_api/`)
- `test_conversion.sh` - OpenAI→Anthropic translation
- `test_tool_resume.sh` - Tool call resumption
- `test_codex_shell_format.sh` - Codex shell command format

#### Streaming (`integration/streaming/`)
- `test_sse_events.sh` - SSE event sequence validation
- `test_detailed.sh` - Detailed streaming behavior
- `test_nonstream_debug.sh` - Non-streaming debug test

#### Duplicate Detection (`integration/duplicate_detection/`)
- `test_detection.sh` - Validates 3/4/5 duplicate warning/stop
- `test_emergency_stop.sh` - Emergency stop behavior

### Fixtures (`fixtures/`)

Sample request data for testing:
- `tool_calls/basic_request.json` - Basic tool call request
- `tool_adapter/` - Tool adapter test cases

### Utilities (`utils/`)

Helper scripts for debugging and validation:
- `capture_sse.sh` - Capture SSE streams for debugging
- `verify_function_call.sh` - Verify function call format
- `verify_incomplete_status.sh` - Verify incomplete status handling
- `verify_output_format.sh` - Verify output format
- `check_tool_format.sh` - Check tool call format
- `diagnose_tool_format.sh` - Diagnose tool format issues

### Configuration (`config/`)

- `test_multiport.sh` - Multi-port configuration test

## Environment Variables

All tests support these environment variables:

- `BASE_URL` - Gateway base URL (default: `http://localhost:8081`)
- `AUTH_HEADER` - Authorization header (default: `Bearer test`)
- `FACADE_PORT` - Facade port (default: `9000`)
- `OPENAI_PORT` - OpenAI port (default: `8082`)
- `ANTHROPIC_PORT` - Anthropic port (default: `8081`)
- `ADMIN_PORT` - Admin port (default: `8080`)

Example:
```bash
BASE_URL=http://localhost:8082 ./integration/tool_calls/test_basic.sh
```

## Requirements

- `curl` - HTTP client
- `jq` - JSON processor
- `python3` - For JSON parsing in some tests
- Running gateway instance

## CI/CD Integration

The test suite is designed for CI/CD integration:

```yaml
# Example GitHub Actions
- name: Run gateway tests
  run: |
    make gd-start
    sleep 2
    ./tests/run_all_tests.sh
```

## Adding New Tests

1. Choose appropriate category directory
2. Create test script with descriptive name
3. Make executable: `chmod +x your_test.sh`
4. Follow naming convention: `test_*.sh`
5. Add to `run_all_tests.sh` if needed
6. Update this README

## Test Output

All tests use consistent output format:
- `🧪` Test header
- `✅` Success
- `❌` Failure
- `⚠️` Warning
- Summary at end

## Troubleshooting

**Tests fail with "connection refused":**
- Ensure gateway is running: `make gd-status`
- Check port configuration: `ss -ltnp | grep gatewayd`

**Auth errors:**
- Disable auth in dev mode: `auth_disabled=true` in config
- Or set proper `TOKLIGENCE_API_KEY`

**Tool call tests fail:**
- Check Anthropic API key: `echo $TOKLIGENCE_ANTHROPIC_API_KEY`
- Verify routes: `TOKLIGENCE_ROUTES=claude*=>anthropic`
