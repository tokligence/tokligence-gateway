# Phase 3: Time-Based Dynamic Rules - Testing Guide

## Overview

Phase 3 implements time-based dynamic rules that automatically adjust scheduler behavior, quotas, and capacity based on time-of-day, day-of-week, and timezone. This guide covers all testing procedures.

## Quick Start

### Run All Phase 3 Tests

```bash
cd tests/integration/scheduler
./run_time_rules_tests.sh
```

### Run Individual Tests

```bash
# Basic functionality test
./run_time_rules_tests.sh basic

# Configuration validation test
./run_time_rules_tests.sh config

# Time window evaluation test
./run_time_rules_tests.sh windows
```

### Run All Scheduler Tests (Including Phase 3)

```bash
cd tests/integration/scheduler
./run_all_tests.sh
```

## Test Files

### 1. `test_time_rules_basic.sh`
**Purpose**: Basic functionality and HTTP API endpoints

**Tests**:
- ✅ Rule engine starts with config file
- ✅ Rules are loaded correctly (3 rule types)
- ✅ `GET /admin/time-rules/status` endpoint
- ✅ `POST /admin/time-rules/apply` endpoint
- ✅ All rules evaluated correctly
- ✅ Logs show rule application

**Expected Output**:
```
✓ HTTP 200 OK
✓ Rule engine is enabled
✓ Rule count is correct: 3
✓ All 3 rules are active
✓ Rules applied successfully
```

### 2. `test_time_rules_config_validation.sh`
**Purpose**: Configuration validation and error handling

**Tests**:
- ✅ Non-existent config file handling
- ✅ Invalid timezone handling
- ✅ Disabled rule engine
- ✅ Valid minimal config
- ✅ Environment variable override

**Expected Output**:
```
✓ Gateway handles missing config file gracefully
✓ Invalid timezone handled gracefully
✓ Disabled rule engine handled correctly
✓ Valid config loads successfully
✓ Environment variable correctly disables engine
```

### 3. `test_time_rules_time_windows.sh`
**Purpose**: Time window evaluation logic

**Tests**:
- ✅ Time-of-day based activation
- ✅ Day-of-week filtering
- ✅ Midnight-wrapping time ranges
- ✅ Timezone handling

**Expected Output**:
```
✓ Rule count is correct: 5
✓ 'Active Rule' is active
✓ 'Inactive Past Rule' is inactive
✓ 'Today Only Rule' is active (day-of-week filter works)
✓ Exactly 2 rules are active (as expected)
```

## Test Configuration

All tests create temporary config files in `/tmp/` and clean up automatically.

### Example Test Config

```ini
[time_rules]
enabled = true
check_interval_sec = 5
default_timezone = UTC

[rule.weights.test]
type = weight_adjustment
name = Test Weight Rule
enabled = true
start_time = 00:00
end_time = 23:59
weights = 100,50,25,10,5,3,2,1,1,1

[rule.quota.test]
type = quota_adjustment
name = Test Quota Rule
enabled = true
start_time = 00:00
end_time = 23:59
quota.test-* = concurrent:10,rps:20,tokens_per_sec:1000

[rule.capacity.test]
type = capacity_adjustment
name = Test Capacity Rule
enabled = true
start_time = 00:00
end_time = 23:59
max_concurrent = 100
max_rps = 200
max_tokens_per_sec = 5000
```

## Live Server Testing

### Manual Live Test

```bash
# 1. Enable time rules in config
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG=config/scheduler_time_rules.ini

# 2. Start gatewayd
make gfr

# 3. Check rule status
curl -s http://localhost:8081/admin/time-rules/status \
  -H "Authorization: Bearer test" | jq .

# 4. Manually trigger rule evaluation
curl -s -X POST http://localhost:8081/admin/time-rules/apply \
  -H "Authorization: Bearer test" | jq .

# 5. Check logs
tail -f logs/gatewayd.log | grep -i "rule"
```

### Expected Live Server Behavior

1. **Startup**: Rule engine loads config and starts
```
[INFO] RuleEngine: Starting (check_interval=1m0s, rules: weight=3 quota=2 capacity=2)
Time-based rule engine started successfully
```

2. **Rule Evaluation**: Every check interval (default 60s)
```
[INFO] RuleEngine: Applying weight rule "External Priority (Nighttime)": weights=[32 32 32 32 32 64 64 64 32 16]
[INFO] RuleEngine: Applying quota rule "Nighttime Quotas": 6 adjustments
```

3. **HTTP Endpoint**: Status returns all rules with active/inactive state
```json
{
  "enabled": true,
  "count": 7,
  "rules": [...]
}
```

## Troubleshooting

### Tests Fail with "Config not found"

**Cause**: Test can't create /tmp/ files
**Solution**: Check /tmp/ permissions, or modify test to use different temp directory

### Tests Fail with "Gateway failed to start"

**Cause**: Port 8081 already in use
**Solution**:
```bash
pkill -f gatewayd
sleep 2
# Re-run test
```

### Time Window Tests Show Unexpected Active/Inactive

**Cause**: Timezone difference between system time and UTC
**Solution**: This is expected - tests account for timezone. Check test output shows correct evaluation based on UTC time.

### Rule Engine Not Loading

**Check**:
1. Config file exists: `ls -la config/scheduler_time_rules.ini`
2. Config syntax valid: Check for INI parsing errors in logs
3. Environment variable set: `echo $TOKLIGENCE_TIME_RULES_ENABLED`

## Integration with CI/CD

### Add to CI Pipeline

```yaml
# .github/workflows/test.yml
- name: Run Phase 3 Integration Tests
  run: |
    cd tests/integration/scheduler
    ./run_time_rules_tests.sh
```

### Docker Testing

```bash
# Build docker image
docker build -t tokligence-gateway:phase3 .

# Run with time rules enabled
docker run -p 8081:8081 \
  -e TOKLIGENCE_TIME_RULES_ENABLED=true \
  -e TOKLIGENCE_TIME_RULES_CONFIG=/app/config/scheduler_time_rules.ini \
  -v $(pwd)/config:/app/config \
  tokligence-gateway:phase3
```

## Test Coverage

- ✅ **Basic Functionality**: 15+ assertions
- ✅ **Time Window Logic**: Complex timezone handling
- ✅ **Configuration**: 5 scenarios, 12+ assertions
- ✅ **Error Handling**: Graceful failures, proper logging
- ✅ **HTTP API**: All endpoints tested

## Related Documentation

- **API Documentation**: `docs/API.md` - Section "Time-Based Dynamic Rules"
- **Design Document**: `docs/design/PHASE3_TIME_BASED_DYNAMIC_RULES.md`
- **Configuration Example**: `config/scheduler_time_rules.ini`
- **Integration Guide**: `tests/integration/scheduler/README.md`

## Support

For issues or questions about Phase 3 testing:
1. Check logs: `tail -f logs/gatewayd.log | grep -i rule`
2. Review test output for specific error messages
3. Verify configuration syntax in INI file
4. Check timezone settings match expectations
