# Firewall Integration Tests

This directory contains integration tests for the Prompt Firewall feature.

## Test Files

### `test_firewall_basic.sh`
A simple, quick test that demonstrates basic PII detection functionality.

**Requirements**:
- Gateway running (no API keys needed)
- Firewall enabled in `config/firewall.ini`

**Usage**:
```bash
./test_firewall_basic.sh
```

**What it tests**:
- Gateway is running
- Firewall configuration is loaded
- PII detection for email and phone number
- Logs show detection results

### `test_pii_detection.sh`
Comprehensive test suite for PII detection across multiple scenarios.

**Requirements**:
- Gateway running
- Firewall enabled in `config/firewall.ini`
- (Optional) API keys in `.env` for real API tests

**Usage**:
```bash
# Run all tests (including real API if keys available)
./test_pii_detection.sh

# Skip real API tests
./test_pii_detection.sh --skip-real-api
```

**What it tests**:
1. Firewall initialization
2. Email + Phone detection
3. Clean requests (no PII)
4. Multiple PII types (Email, Phone, SSN)
5. Real API integration (if keys available)

## Running Tests in CI/CD

### GitHub Actions

The tests are designed to work in CI/CD environments:

```yaml
# .github/workflows/firewall-tests.yml
name: Firewall Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Build gateway
        run: make build

      - name: Create firewall config
        run: |
          mkdir -p config
          cp examples/firewall/configs/firewall.ini config/

      - name: Start gateway
        run: |
          make gds
          sleep 5  # Wait for gateway to start

      - name: Run basic firewall test (no API keys needed)
        run: ./tests/integration/firewall/test_firewall_basic.sh

      # Only run real API tests if secrets are available
      - name: Run comprehensive tests
        if: ${{ secrets.TOKLIGENCE_OPENAI_API_KEY != '' }}
        env:
          TOKLIGENCE_OPENAI_API_KEY: ${{ secrets.TOKLIGENCE_OPENAI_API_KEY }}
        run: ./tests/integration/firewall/test_pii_detection.sh

      # Run without real API if no secrets
      - name: Run tests without real API
        if: ${{ secrets.TOKLIGENCE_OPENAI_API_KEY == '' }}
        run: ./tests/integration/firewall/test_pii_detection.sh --skip-real-api
```

### API Key Detection

Both test scripts automatically detect API keys:
- If `.env` file exists with valid `TOKLIGENCE_OPENAI_API_KEY`, real API tests run
- If no keys found, real API tests are skipped
- Use `--skip-real-api` flag to explicitly skip real API tests

## Prerequisites

### Minimal Setup (Basic Test)
```bash
# 1. Build gateway
make build

# 2. Copy firewall config
cp examples/firewall/configs/firewall.ini config/

# 3. Start gateway
make gds

# 4. Run test
./tests/integration/firewall/test_firewall_basic.sh
```

### Full Setup (All Tests)
```bash
# 1-3. Same as above

# 4. Create .env with API keys (optional)
cat > .env <<EOF
TOKLIGENCE_OPENAI_API_KEY=sk-proj-...
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-...
EOF

# 5. Run comprehensive tests
./tests/integration/firewall/test_pii_detection.sh
```

## Expected Output

### Basic Test Success
```
==========================================
Firewall Basic Test
==========================================

Checking gateway... ✓
Checking firewall config... ✓

Test 1: Sending request with PII (email + phone)...
----------------------------------------------
Request sent

Checking logs for PII detections...
----------------------------------------------
Looking in: logs/dev-gatewayd-2025-11-21.log

[gatewayd/http][DEBUG] firewall.detection location=input type=pii severity=medium details={"confidence":0.95,"pattern":"email","pii_type":"EMAIL"}
[gatewayd/http][DEBUG] firewall.detection location=input type=pii severity=medium details={"confidence":0.9,"pattern":"phone_us","pii_type":"PHONE"}
[gatewayd/http][DEBUG] firewall.monitor location=input pii_count=2 types=[]

==========================================
Test complete!
==========================================
```

### Comprehensive Test Success
```
============================================
Firewall PII Detection Integration Test
============================================

[INFO] Checking prerequisites...
[PASS] Gateway is running
[PASS] Firewall config found
[PASS] Log file found at logs/dev-gatewayd-2025-11-21.log
[PASS] OpenAI API key found in .env

[INFO] Test 1: Checking firewall initialization in logs...
[PASS] Firewall initialized successfully (mode=monitor, filters=1)

[INFO] Test 2: Testing PII detection (Email + Phone)...
[PASS] Request processed successfully
[PASS] Email PII detected in logs
[PASS] Phone PII detected in logs
[PASS] PII count recorded in monitor logs

...

============================================
Test Summary
============================================
Tests run:    5
Tests passed: 5
Tests failed: 0

[PASS] All tests passed!
```

## Troubleshooting

### Gateway Not Running
```
[FAIL] Gateway is not running at http://localhost:8081
Start gateway with: make gds
```

**Solution**: Start the gateway with `make gds` or `make gfr`

### Firewall Not Enabled
```
[FAIL] Firewall disabled in configuration
```

**Solution**:
1. Check `config/firewall.ini` exists
2. Verify `enabled = true` in the `[prompt_firewall]` section
3. Restart gateway: `make gfr`

### No PII Detected in Logs
```
[FAIL] Email PII not detected in logs
```

**Possible causes**:
1. Log level not set to DEBUG:
   - Set `TOKLIGENCE_LOG_LEVEL=debug` in `.env`
   - Or in `config/setting.ini`: `log_level=debug`

2. Firewall filters not enabled:
   - Check `config/firewall.ini`
   - Verify `[firewall_input_filters]` section has `filter_pii_regex_enabled = true`

3. Wrong log file:
   - Check `logs/` directory for actual log file name
   - May be dated file like `dev-gatewayd-2025-11-21.log`

### Real API Tests Failing
```
[FAIL] Real API request failed: invalid_api_key
```

**Solution**:
1. Verify API key in `.env` is valid
2. Check key has not expired
3. Use `--skip-real-api` flag to skip these tests

## Adding New Tests

To add a new firewall test:

1. Create a new test script: `test_<feature>.sh`
2. Follow the pattern in existing tests:
   - Check prerequisites
   - Run test cases
   - Verify results in logs
   - Report pass/fail

3. Make it executable:
   ```bash
   chmod +x test_<feature>.sh
   ```

4. Document it in this README

## See Also

- [Firewall Documentation](../../../docs/PROMPT_FIREWALL.md)
- [Deployment Guide](../../../examples/firewall/DEPLOYMENT_GUIDE.md)
- [Performance Tuning](../../../examples/firewall/PERFORMANCE_TUNING.md)
