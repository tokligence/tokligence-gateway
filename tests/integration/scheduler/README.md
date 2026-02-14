# Scheduler Integration Tests

Comprehensive integration test suite for the priority-based request scheduler.

## Test Coverage

### 1. Backward Compatibility (`test_scheduler_disabled.sh`)

**Purpose**: Verify gateway functions normally when scheduler is disabled

**Tests**:
- ✅ Gateway starts without scheduler enabled
- ✅ Health endpoint responds correctly
- ✅ Multiple concurrent requests succeed immediately
- ✅ No scheduler-related errors in logs
- ✅ No queueing overhead

**Configuration**:
```bash
TOKLIGENCE_SCHEDULER_ENABLED=false
```

**Expected Behavior**: All requests processed immediately through native gateway, no scheduler initialization

---

### 2. Priority Levels (`test_priority_levels.sh`)

**Purpose**: Verify all 10 priority levels work correctly

**Tests**:
- ✅ Scheduler enabled with 10 priority levels (P0-P9)
- ✅ Hybrid policy configured (P0 strict, P1-P9 WFQ)
- ✅ Requests submitted across all priority tiers
- ✅ All priority levels processed successfully
- ✅ Capacity tracking operational

**Priority Tiers Tested**:
- P0 (Critical)
- P2 (High)
- P5 (Normal) - Default
- P7 (Low)
- P9 (Background)

**Configuration**:
```bash
TOKLIGENCE_SCHEDULER_ENABLED=true
TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS=10
TOKLIGENCE_SCHEDULER_POLICY=hybrid
```

**Expected Behavior**: Scheduler initializes, accepts requests at all priority levels, demonstrates capacity tracking

---

### 3. Capacity Limits (`test_capacity_limits.go`)

**Purpose**: Verify capacity enforcement and automatic queueing

**Tests**:
- ✅ Concurrent request limit enforcement (5 max concurrent)
- ✅ Automatic queueing when capacity reached
- ✅ Request dequeuing as capacity becomes available
- ✅ Token rate limiting (500 tokens/sec limit)
- ✅ Complete request lifecycle (submit → queue → execute → release)

**Test Scenarios**:
1. **Concurrent Limit Test**: Submit 20 requests with max_concurrent=5
   - First 5 accepted immediately
   - Remaining 15 queued
   - All 20 eventually complete

2. **Token Rate Test**: Submit high-token requests (200 tokens each)
   - Hits 500 tokens/sec limit quickly
   - Requests automatically queued
   - All complete as token budget replenishes

**Configuration**:
```go
MaxConcurrent: 5
MaxTokensPerSec: 500
MaxRPS: 10
```

**Expected Behavior**:
- Some requests queued due to capacity limits
- All requests eventually complete
- No rejections (sufficient queue depth)

---

### 4. WFQ Fairness (`test_wfq_fairness.go`)

**Purpose**: Verify Weighted Fair Queuing prevents starvation

**Tests**:
- ✅ No starvation - all priorities complete all requests
- ✅ Higher weights → better service (lower average wait time)
- ✅ Weighted bandwidth allocation working
- ✅ P0 requests processed with strict priority
- ✅ P1-P9 get fair share via WFQ

**Test Scenario**:
- Submit 10 requests × 10 priority levels = 100 total requests
- Very low capacity (max_concurrent=3) to force heavy queueing
- Measure completion order and wait times
- Verify no priority level is starved

**Weights** (exponential):
```
P0: 512 (2^9)
P1: 256 (2^8)
P2: 128 (2^7)
...
P9: 1   (2^0)
```

**Expected Behavior**:
- All 100 requests complete
- Higher priority → lower average wait time
- No starvation (all priorities complete)
- P0 may complete first (depending on policy)

---

### 5. Queue Timeout & Rejection (`test_queue_timeout.go`)

**Purpose**: Verify error handling and graceful degradation

**Tests**:
- ✅ Queue timeout - requests waiting too long are rejected
- ✅ Queue depth limit - new requests rejected when queue full
- ✅ Context length limit - oversized requests rejected immediately
- ✅ Proper error messages returned

**Test Scenarios**:

1. **Queue Timeout Test**:
   - Short timeout: 2 seconds
   - Slow processing: 1 second per request
   - Submit more requests than can complete in timeout window
   - Verify some requests timeout with proper error message

2. **Queue Depth Limit Test**:
   - Small queue depth: 5 per priority
   - Rapid submission: 15 requests (3× queue depth)
   - Verify queue full rejections occur

3. **Context Length Limit Test**:
   - Max context: 1000 tokens
   - Submit requests with 2000 tokens
   - Verify immediate rejection at submit time

**Configuration**:
```go
QueueTimeout: 2 seconds
MaxQueueDepth: 5
MaxContextLength: 1000
```

**Expected Behavior**:
- Timeout errors for requests waiting > 2s
- Queue full errors for excess submissions
- Context limit errors for oversized requests
- All errors have clear, actionable messages

---

## Running Tests

### Run All Tests
```bash
./tests/integration/scheduler/run_all_tests.sh
```

**Output**: Colored summary with pass/fail status for each test suite

### Run Individual Tests

**Bash tests**:
```bash
bash tests/integration/scheduler/test_scheduler_disabled.sh
bash tests/integration/scheduler/test_priority_levels.sh
```

**Go tests**:
```bash
go run tests/integration/scheduler/test_capacity_limits.go
go run tests/integration/scheduler/test_wfq_fairness.go
go run tests/integration/scheduler/test_queue_timeout.go
```

---

## Test Requirements

### Environment Setup
- Go 1.24+
- Gateway built (`make bgd`)
- Config files in place (`config/scheduler.ini`, `config/gateway.ini`)

### Environment Variables (optional overrides)
```bash
TOKLIGENCE_SCHEDULER_ENABLED=true|false
TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS=10
TOKLIGENCE_SCHEDULER_POLICY=strict|wfq|hybrid
TOKLIGENCE_SCHEDULER_MAX_CONCURRENT=100
TOKLIGENCE_SCHEDULER_MAX_TOKENS_PER_SEC=10000
```

---

## Expected Results

When all tests pass:
```
Total Tests:   5
Passed:        5
Failed:        0
Skipped:       0

✓ ALL TESTS PASSED

Scheduler is ready for production use with:
  ✓ Backward compatibility (can be disabled)
  ✓ 10-level priority queue
  ✓ Capacity management (tokens/sec, RPS, concurrent)
  ✓ WFQ fairness (no starvation)
  ✓ Proper error handling (timeout, queue full, context limit)
```

---

## Debugging Failed Tests

### Test 1 (Scheduler Disabled) Fails
**Symptom**: Gateway doesn't start or health check fails

**Debug Steps**:
1. Check gateway logs: `cat /tmp/gateway_scheduler_disabled.log`
2. Verify no API keys required: `TOKLIGENCE_AUTH_DISABLED=true`
3. Check port availability: `lsof -i :8081`

### Test 2 (Priority Levels) Fails
**Symptom**: Scheduler not enabled or wrong priority count

**Debug Steps**:
1. Check config loading: `grep scheduler config/scheduler.ini`
2. Verify environment variables override: `echo $TOKLIGENCE_SCHEDULER_ENABLED`
3. Check demo output: `cat /tmp/scheduler_priority_test.log`

### Test 3 (Capacity Limits) Fails
**Symptom**: No queuing or all requests rejected

**Debug Steps**:
1. Check if limits too high/low
2. Verify concurrent limit enforcement
3. Check debug logs for capacity calculations

### Test 4 (WFQ Fairness) Fails
**Symptom**: Starvation detected or wrong completion order

**Debug Steps**:
1. Check if policy is WFQ (not strict)
2. Verify weights configured correctly
3. Look for deficit tracking logs

### Test 5 (Timeout/Rejection) Fails
**Symptom**: No timeouts or rejections

**Debug Steps**:
1. Verify timeout duration is short enough
2. Check queue depth limits
3. Confirm context length validation

---

## Performance Notes

### Test Execution Time

- **test_scheduler_disabled.sh**: ~5 seconds (starts/stops gateway)
- **test_priority_levels.sh**: ~3 seconds (quick demo)
- **test_capacity_limits.go**: ~10 seconds (20 concurrent + 10 token tests)
- **test_wfq_fairness.go**: **~2-3 minutes** (100 requests with queueing)
- **test_queue_timeout.go**: ~30 seconds (timeout waits + multiple scenarios)

**Total Suite**: ~3-4 minutes

### Speeding Up Tests

For faster iterations during development:

1. Run only quick tests:
   ```bash
   bash tests/integration/scheduler/test_priority_levels.sh
   go run tests/integration/scheduler/test_capacity_limits.go
   ```

2. Reduce WFQ test size (edit `test_wfq_fairness.go`):
   ```go
   const requestsPerPriority = 3  // instead of 10
   ```

3. Skip gateway test (already tested in unit tests):
   ```bash
   # Only run Go tests
   go run tests/integration/scheduler/test_*.go
   ```

---

## Test Maintenance

### Adding New Tests

1. Create test file in `tests/integration/scheduler/`
2. Follow naming convention: `test_<feature>.{sh|go}`
3. Make bash scripts executable: `chmod +x test_*.sh`
4. Add to `run_all_tests.sh` in appropriate test suite
5. Update this README with test description

### Modifying Existing Tests

1. Preserve test coverage - don't remove assertions
2. Update expected results if behavior changes
3. Keep test execution time reasonable (<5 min total)
4. Document any new configuration requirements

---

## CI/CD Integration

To run in CI pipeline:

```yaml
test:
  script:
    - make bgd
    - ./tests/integration/scheduler/run_all_tests.sh
  timeout: 10m
```

**Note**: WFQ fairness test may be slow in CI environments. Consider reducing `requestsPerPriority` for CI or running it separately as a longer stress test.

---

## Future Test Enhancements

Potential additions for more comprehensive testing:

1. **Stress Testing**: High load scenarios (1000+ concurrent requests)
2. **Latency Testing**: Measure p50/p90/p99 wait times per priority
3. **Policy Comparison**: A/B test strict vs WFQ vs hybrid
4. **Dynamic Configuration**: Test config hot-reload
5. **Metric Validation**: Verify Prometheus metrics accuracy
6. **Per-Account Limits**: Test multi-tenant isolation
7. **Graceful Degradation**: Test scheduler under resource exhaustion

---

## Related Documentation

- **Scheduler Design**: `/home/alejandroseaah/tokligence/arc_design/v20251123/`
- **Configuration Guide**: `config/scheduler.ini` (inline comments)
- **API Documentation**: `docs/API.md` (Priority Scheduling section)
- **Unit Tests**: `internal/scheduler/scheduler_test.go`

---

**Last Updated**: 2025-11-23
**Version**: v0.3.0 (Priority Scheduler MVP)
