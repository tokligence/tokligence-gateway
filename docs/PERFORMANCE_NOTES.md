# Scheduler Performance Analysis and Optimization Notes

## Current Performance Characteristics

### Lock Contention Points

**Capacity Guardian** (`internal/scheduler/capacity.go`):
- `sync.Mutex` lock on EVERY request check (`CanAccept()`)
- Lock held during all capacity calculations
- **Impact**: Potential bottleneck under high concurrency

**Priority Queue** (`internal/scheduler/priority.go`):
- Per-queue mutex locks for enqueue/dequeue
- 10 separate mutexes (one per priority level)
- **Impact**: Minimal - locks are per-priority, not global

**Statistics** (`scheduler.go`):
- Mutex lock for every stat update (totalScheduled, totalRejected)
- **Impact**: Minimal - quick integer increments

### Optimization Opportunities

#### 1. Fast Path for Immediate Acceptance (已实现)
```go
// scheduler.go:85
canAccept, reason := s.capacityGuardian.CheckAndReserve(req)
if canAccept {
    // ✓ Immediate return without queueing
    return nil
}
```
**When scheduler disabled**: Zero overhead (bypass completely)
**When capacity available**: Single mutex lock + immediate return
**When capacity full**: Enqueue + background worker processes

#### 2. Reduce Logging Overhead

**Current State**:
- DEBUG logs on EVERY request (capacity check, queue operations)
- INFO logs on submit/accept/reject
- Multiple log.Printf() calls per request

**Recommendation**:
```go
// Only log when DEBUG level is explicitly enabled
if s.isDebug {
    log.Printf(...)
}
```

**Already Implemented in HTTP handlers**:
```go
if s.isDebug() && s.logger != nil {
    s.logger.Printf(...)
}
```

#### 3. Background Worker Polling Interval

**Current**: 100ms polling (`time.NewTicker(100 * time.Millisecond)`)

**Trade-offs**:
- Lower interval (e.g., 10ms): Lower latency, higher CPU usage
- Higher interval (e.g., 500ms): Lower CPU usage, higher latency for queued requests

**Recommendation**:
- Keep 100ms as default (good balance)
- Make configurable via `scheduler_poll_interval_ms` config
- Consider event-driven approach with channels (future enhancement)

#### 4. Atomic Counters for Statistics

**Current**: mutex-protected integer counters

**Optimization**:
```go
import "sync/atomic"

type Stats struct {
    totalScheduled atomic.Int64
    totalRejected  atomic.Int64
}

// Lock-free increment
atomic.AddInt64(&s.stats.totalScheduled, 1)
```

**Impact**: Reduces lock contention for stats updates

### Benchmark Results (Future)

To measure actual impact, run:
```bash
go test -bench=. ./internal/scheduler -benchmem
```

Expected results for well-optimized scheduler:
- Immediate acceptance: < 1μs per request
- Queueing path: < 10μs per request
- Lock contention: Minimal with < 1000 concurrent requests

### Configuration for High-Performance Scenarios

**High-Concurrency Setup** (many small requests):
```ini
scheduler_max_concurrent = 500
scheduler_max_rps = 1000
scheduler_max_tokens_per_sec = 100000
scheduler_poll_interval_ms = 50  # Future: faster polling
```

**Low-Latency Setup** (prioritize response time):
```ini
scheduler_policy = strict  # Skip WFQ overhead for P0
scheduler_priority_levels = 5  # Fewer queues = less overhead
```

### Monitoring Recommendations

**Key Metrics to Track**:
1. **Scheduler Overhead**: Time spent in `Submit()` call
2. **Queue Wait Time**: P50/P90/P99 per priority level
3. **Lock Wait Time**: Time waiting for capacity mutex
4. **Rejection Rate**: % of requests rejected vs accepted

**Prometheus Metrics** (future enhancement):
```
tokligence_scheduler_submit_duration_seconds{priority="0"}
tokligence_scheduler_queue_wait_seconds{priority="5"}
tokligence_scheduler_capacity_lock_wait_seconds
tokligence_scheduler_requests_total{status="accepted|rejected|queued"}
```

### Known Limitations

1. **Single-Node Only**: Current scheduler only works within one gateway process
2. **In-Memory State**: Queue state lost on restart
3. **No Backpressure**: Clients receive 503/429 but no retry-after hints
4. **Token Estimation**: Rough estimate (4 chars/token) may be inaccurate

### Future Optimizations

1. **Lock-Free Queue**: Consider using `sync/atomic` with ring buffer
2. **Event-Driven Dequeue**: Replace polling with channel notifications
3. **Per-Account Limits**: Add account-level rate limiting
4. **Distributed Scheduler**: Use Redis/etcd for multi-node coordination

---

**Last Updated**: 2025-11-23
**Version**: v0.3.0 (Scheduler MVP)
