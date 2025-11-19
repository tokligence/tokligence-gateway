# Gemini Integration Tests

This directory contains integration tests for the Gemini API proxy functionality.

## Prerequisites

1. **Gateway Running**: The gateway must be running on port 8084 (or custom port)
2. **API Key Configured**: Set `TOKLIGENCE_GEMINI_API_KEY` environment variable
3. **Dependencies**: `curl`, `jq`, and `bash` must be available

## Quick Start

```bash
# Start the gateway
make gfr

# Run all Gemini tests
./tests/integration/gemini/run_all_gemini_tests.sh
```

## Test Suites

### 1. test_native_api.sh

Tests native Gemini API endpoints:
- List models (`GET /v1beta/models`)
- Get model info (`GET /v1beta/models/{model}`)
- Count tokens (`POST /v1beta/models/{model}:countTokens`)
- Generate content non-streaming (`POST /v1beta/models/{model}:generateContent`)
- Generate content streaming (`POST /v1beta/models/{model}:streamGenerateContent`)

**Run individually:**
```bash
./tests/integration/gemini/test_native_api.sh
```

### 2. test_openai_compat.sh

Tests OpenAI-compatible endpoints:
- Chat completions non-streaming
- Chat completions streaming
- Multi-turn conversations
- Response structure validation

**Run individually:**
```bash
./tests/integration/gemini/test_openai_compat.sh
```

### 3. test_error_handling.sh

Tests error handling scenarios:
- Invalid model names
- Empty model names
- Invalid JSON bodies
- Missing required fields
- Invalid API methods
- Large request handling

**Run individually:**
```bash
./tests/integration/gemini/test_error_handling.sh
```

## Configuration

### Environment Variables

```bash
# Gateway URL (default: http://localhost:8084)
export GATEWAY_URL=http://localhost:8084

# Test model to use (default: gemini-2.0-flash-exp)
export TEST_MODEL=gemini-2.0-flash-exp

# Gemini API key (required)
export TOKLIGENCE_GEMINI_API_KEY=your_api_key_here
```

### Custom Port

If running gateway on a different port:

```bash
GATEWAY_URL=http://localhost:9000 ./tests/integration/gemini/run_all_gemini_tests.sh
```

### Different Model

To test with a different Gemini model:

```bash
TEST_MODEL=gemini-1.5-pro ./tests/integration/gemini/run_all_gemini_tests.sh
```

## Interpreting Results

### Test Outcomes

- **✓ PASS** (Green): Test passed successfully
- **✗ FAIL** (Red): Test failed unexpectedly
- **⊘ SKIP** (Yellow): Test skipped (usually due to quota limits)

### Common Issues

**Quota Exceeded (HTTP 429)**

Tests automatically skip when quota is exceeded. This is expected for free tier API keys.

**Solution:**
- Wait for quota reset (typically 1 minute)
- Upgrade to a paid Gemini API tier
- Use `TEST_MODEL=gemini-1.5-flash` for higher quotas

**Gateway Not Reachable**

**Solution:**
```bash
# Check if gateway is running
curl http://localhost:8084/health

# Start gateway if needed
make gfr

# Check logs
tail -f logs/gateway.log
```

**API Key Not Configured**

**Solution:**
```bash
export TOKLIGENCE_GEMINI_API_KEY=your_api_key_here
make gfr  # Restart gateway to pick up new env var
```

## Expected Output

Successful test run:
```
========================================
  Gemini Integration Test Suite
========================================

Gateway URL: http://localhost:8084
Test Model: gemini-2.0-flash-exp

Checking gateway connectivity...
✓ Gateway is reachable

Checking configuration...
✓ TOKLIGENCE_GEMINI_API_KEY is configured

========================================
Running: test_native_api.sh
========================================

Test 1: List models
✓ PASS: List models returned valid response
...

=== Test Summary ===
Passed: 5
Failed: 0

✓ All tests passed!
```

## Troubleshooting

### Debug Mode

Enable verbose output for debugging:

```bash
set -x
./tests/integration/gemini/test_native_api.sh
```

### Manual API Testing

Test individual endpoints manually:

```bash
# Test health endpoint
curl http://localhost:8084/health

# Test list models
curl http://localhost:8084/v1beta/models | jq

# Test generate content
curl -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:generateContent' \
  -H 'Content-Type: application/json' \
  -d '{"contents":[{"parts":[{"text":"Hello"}]}]}' | jq
```

### Check Gateway Logs

```bash
# View recent logs
tail -n 100 logs/gateway.log

# Follow logs in real-time
tail -f logs/gateway.log
```

## CI/CD Integration

These tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Run Gemini Integration Tests
  env:
    TOKLIGENCE_GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
  run: |
    make gds
    sleep 5
    ./tests/integration/gemini/run_all_gemini_tests.sh
```

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Use color codes for output (GREEN/RED/YELLOW)
3. Handle quota exceeded errors gracefully
4. Include descriptive test names
5. Update this README with new test descriptions

## Related Documentation

- [Gemini Integration Guide](../../../docs/gemini-integration.md)
- [Gateway Quick Start](../../../docs/QUICK_START.md)
- [Gemini API Documentation](https://ai.google.dev/gemini-api/docs)
