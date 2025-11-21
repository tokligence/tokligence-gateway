# Translation Library Integration Tests

This directory contains integration tests for the translation library advanced features.

## Test Files

### test_p0_advanced_features.sh

Tests P0 features of the translation library integration:
- `web_search_options` field support
- `reasoning_effort` field support
- `thinking` configuration support
- Rich usage tracking (cache tokens, reasoning tokens)
- Backward compatibility
- Error handling

**Requirements:**
- Gateway running on `localhost:8081` (or set `BASE_URL` env var)
- `TOKLIGENCE_ANTHROPIC_API_KEY` configured in `.env`

**Usage:**

```bash
# Run from repository root
./tests/integration/translation_lib/test_p0_advanced_features.sh

# Or with custom base URL
BASE_URL=http://localhost:9000 ./tests/integration/translation_lib/test_p0_advanced_features.sh
```

**Test Coverage:**

1. ✅ Backward compatibility - existing requests work unchanged
2. ✅ `reasoning_effort` field acceptance and processing
3. ✅ `thinking` configuration field acceptance
4. ✅ Multiple new fields combined in single request
5. ✅ Rich usage tracking validation
6. ⚠️ Responses API compatibility (optional, may be skipped)
7. ✅ Error handling for invalid values
8. ✅ Null/empty optional fields handling

**Exit Codes:**
- `0` - All tests passed
- `1` - One or more tests failed

**CI/CD Integration:**

The workflow `.github/workflows/p0-integration-tests.yml` is configured to:
- **Skip gracefully** if API keys are not configured
- Always run compilation tests (no API keys needed)
- Run integration tests only if `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` secrets are set

To enable integration tests in CI/CD:
1. Go to repository Settings → Secrets and variables → Actions
2. Add secrets:
   - `ANTHROPIC_API_KEY` (optional)
   - `OPENAI_API_KEY` (optional)

Example workflow:
```yaml
# .github/workflows/integration-tests.yml
- name: Run P0 Translation Features Tests
  run: |
    # Start gateway
    make gfr &
    sleep 5

    # Run tests (will skip if no API keys)
    ./tests/integration/translation_lib/test_p0_advanced_features.sh
```

**Note**: If you're using this in a public repository or fork, you may want to disable these tests or use your own API keys in secrets.

## Adding New Tests

When adding new translation library features, follow this pattern:

```bash
# Test N: [Feature description]
echo "Test N: [Feature name]"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "[new_field]": "[value]"
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: [Success message]"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: [Failure message]"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""
```

## Test Philosophy

1. **Backward Compatible**: Always test that existing functionality still works
2. **Provider Agnostic**: Test gateway behavior, not provider specifics
3. **CI/CD Ready**: Fast, reliable, no manual intervention needed
4. **Clear Output**: Use color coding and clear pass/fail messages
5. **Timeout Protected**: All curl requests have timeouts
6. **Graceful Degradation**: Use SKIP status for optional features

## Related Documentation

- `docs/endpoint-translation-todo.md` - Full roadmap and feature status
- `docs/P0_INTEGRATION_TESTING.md` - Detailed P0 testing guide
- Translation library: https://github.com/tokligence/openai-anthropic-endpoint-translation
