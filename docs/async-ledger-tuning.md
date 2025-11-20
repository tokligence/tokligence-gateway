# Async Ledger Batch Writer Tuning Guide

## Overview

The async ledger batch writer uses channels and goroutines to perform non-blocking database writes, preventing ledger operations from blocking HTTP responses. This document explains the tuning parameters and provides recommended configurations for different throughput scenarios.

## Architecture

```
HTTP Request → Record() → Channel (buffered) → Multiple Worker Goroutines → Batch INSERT to PostgreSQL
     ↓
  Immediate Response
```

- **Non-blocking writes**: HTTP requests return immediately without waiting for database INSERT
- **Batching**: Entries are grouped and written in batches to reduce database roundtrips
- **Parallel workers**: Multiple goroutines process batches concurrently
- **Time-based flushing**: Ensures entries are written even if batch size isn't reached

## Configuration Parameters

### 1. `ledger_async_batch_size` (Env: `TOKLIGENCE_LEDGER_ASYNC_BATCH_SIZE`)

**Purpose**: Maximum number of entries per batch before forcing a database write.

- **Default**: 100
- **Impact**:
  - Larger batches → Fewer database roundtrips, higher throughput
  - Smaller batches → Lower latency, more real-time
- **Trade-off**: Large batches may delay writes if traffic is bursty

### 2. `ledger_async_flush_ms` (Env: `TOKLIGENCE_LEDGER_ASYNC_FLUSH_MS`)

**Purpose**: Maximum time in milliseconds between flushes (safety net for low-traffic periods).

- **Default**: 1000ms (1 second)
- **Impact**:
  - Shorter interval → More real-time writes, higher database load
  - Longer interval → May delay writes during low-traffic periods
- **Trade-off**: Must balance latency vs database overhead

### 3. `ledger_async_buffer_size` (Env: `TOKLIGENCE_LEDGER_ASYNC_BUFFER_SIZE`)

**Purpose**: Channel buffer size - maximum entries queued in memory before blocking/dropping.

- **Default**: 10,000
- **Impact**:
  - Larger buffer → Handles traffic bursts better
  - Smaller buffer → Lower memory usage, less data loss on crash
- **Critical**: Buffer must be sized for peak burst traffic!

**Formula**: `buffer_size ≥ peak_qps × burst_duration_seconds`

Example: For 1M QPS with 1-second burst tolerance:
```
buffer_size ≥ 1,000,000 × 1 = 1,000,000 entries
```

### 4. `ledger_async_num_workers` (Env: `TOKLIGENCE_LEDGER_ASYNC_NUM_WORKERS`)

**Purpose**: Number of parallel goroutines writing batches to the database.

- **Default**: 1
- **Impact**:
  - More workers → Higher throughput, better parallelism
  - Fewer workers → Lower database connection usage
- **Optimal**: `workers = min(cpu_cores, database_max_connections / 10)`

## Recommended Configurations

### Low Traffic (< 100 QPS)

```ini
ledger_async_batch_size=50
ledger_async_flush_ms=2000
ledger_async_buffer_size=5000
ledger_async_num_workers=1
```

**Characteristics**:
- Low latency (max 2s delay)
- Minimal resource usage
- Single worker sufficient

---

### Medium Traffic (100-1,000 QPS)

```ini
ledger_async_batch_size=100
ledger_async_flush_ms=1000
ledger_async_buffer_size=10000
ledger_async_num_workers=2
```

**Characteristics**:
- Balanced latency/throughput
- 10s burst tolerance at 1K QPS
- 2 workers for parallelism

---

### High Traffic (1K-10K QPS)

```ini
ledger_async_batch_size=500
ledger_async_flush_ms=500
ledger_async_buffer_size=50000
ledger_async_num_workers=5
```

**Characteristics**:
- 5s burst tolerance at 10K QPS
- 500ms max latency
- 5 workers for high throughput

**Database connections**: ~5-10 active connections

---

### Very High Traffic (10K-100K QPS)

```ini
ledger_async_batch_size=2000
ledger_async_flush_ms=200
ledger_async_buffer_size=200000
ledger_async_num_workers=10
```

**Characteristics**:
- 2s burst tolerance at 100K QPS
- 200ms max latency
- 10 workers for parallel processing

**Database connections**: ~10-20 active connections

**Database tuning**:
- Increase PostgreSQL `max_connections` to 200+
- Enable connection pooling (PgBouncer)
- Optimize INSERT performance with indexes

---

### Ultra High Traffic (100K-1M QPS)

```ini
ledger_async_batch_size=10000
ledger_async_flush_ms=100
ledger_async_buffer_size=1000000
ledger_async_num_workers=50
```

**Characteristics**:
- 1s burst tolerance at 1M QPS
- 100ms max latency
- 50 workers for massive parallelism

**Database connections**: ~50-100 active connections

**Critical requirements**:
- High-performance PostgreSQL instance (16+ cores, 64GB+ RAM)
- SSD/NVMe storage for fast writes
- Connection pooler (PgBouncer/pgpool) mandatory
- Consider partitioning usage_entries table by time
- Monitor `pg_stat_activity` for connection saturation

**Alternative**: Consider using a write-optimized database (Cassandra, ScyllaDB) or message queue (Kafka) for ledger at this scale.

---

### Extreme Scale (1M+ QPS)

At this scale, the async batch writer may become a bottleneck. Consider:

1. **Sharding**: Run multiple gateway instances with separate databases
2. **Message Queue**: Replace ledger with Kafka/NATS, process asynchronously
3. **Time-series DB**: Use ClickHouse/TimescaleDB optimized for append-only workloads
4. **Sampling**: Only record a percentage of requests (e.g., 1% sampling)

## Performance Monitoring

Monitor these metrics to tune parameters:

### Channel Saturation
```go
// WARNING in logs indicates channel full
[async-ledger] WARNING: channel full, dropping entry
```
**Action**: Increase `ledger_async_buffer_size` or `num_workers`

### Flush Frequency
```go
[async-ledger] worker-0 flushed 100/100 entries in 45ms (2222 entries/sec)
```
**Metrics to watch**:
- If batch always full → increase `batch_size`
- If flush time > 100ms → optimize database or reduce `batch_size`
- If entries/sec < target QPS → increase `num_workers`

### Database Connections
```sql
-- PostgreSQL: Check active connections
SELECT count(*) FROM pg_stat_activity WHERE state = 'active';
```
**Action**: If connections == `num_workers`, workers are I/O bound

## Data Loss Risk

**Crash scenarios**:
- Process crash → Lose up to `buffer_size` entries
- Graceful shutdown → All entries flushed (no loss)

**Mitigation**:
1. Use systemd/supervisor for auto-restart
2. Enable PostgreSQL replication for database durability
3. For critical billing data, consider synchronous writes or external audit log

## Environment Variable Override Example

For production deployment at 50K QPS:

```bash
export TOKLIGENCE_LEDGER_ASYNC_BATCH_SIZE=5000
export TOKLIGENCE_LEDGER_ASYNC_FLUSH_MS=200
export TOKLIGENCE_LEDGER_ASYNC_BUFFER_SIZE=500000
export TOKLIGENCE_LEDGER_ASYNC_NUM_WORKERS=20

./bin/gatewayd
```

## Benchmarking

To measure actual throughput:

```bash
# Watch async ledger logs
tail -f /tmp/tokligence-gateway-daemon.out | grep "async-ledger"

# Example output:
[async-ledger] worker-3 flushed 5000/5000 entries in 123ms (40650 entries/sec)
[async-ledger] worker-7 flushed 5000/5000 entries in 118ms (42372 entries/sec)
```

**Target**: `total_throughput = num_workers × entries_per_sec`

For 1M QPS: `50 workers × 20,000 entries/sec = 1M entries/sec`

## Troubleshooting

### Problem: High P99 latency despite async writes

**Cause**: Channel buffer full, causing backpressure

**Solution**:
1. Increase `ledger_async_buffer_size`
2. Increase `num_workers`
3. Check database slow query log

### Problem: Entries being dropped

**Symptoms**: `WARNING: channel full, dropping entry` in logs

**Solution**:
1. Immediately increase `ledger_async_buffer_size` (e.g., 10x)
2. Add more `num_workers`
3. Check PostgreSQL performance (slow INSERTs)

### Problem: Database connection pool exhausted

**Symptoms**: `pq: too many clients already` or similar errors

**Solution**:
1. Reduce `num_workers`
2. Increase `db_max_open_conns`
3. Deploy PgBouncer connection pooler

## Summary

| QPS Range | Batch Size | Flush (ms) | Buffer Size | Workers |
|-----------|------------|------------|-------------|---------|
| < 100     | 50         | 2000       | 5,000       | 1       |
| 100-1K    | 100        | 1000       | 10,000      | 2       |
| 1K-10K    | 500        | 500        | 50,000      | 5       |
| 10K-100K  | 2,000      | 200        | 200,000     | 10      |
| 100K-1M   | 10,000     | 100        | 1,000,000   | 50      |
| 1M+       | Consider sharding or alternative architecture |

**Key Formula**: `buffer_size ≥ peak_qps × (batch_size / workers / db_throughput)`
