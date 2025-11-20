# Tokligence Gateway Performance Benchmark

Comprehensive performance testing framework to benchmark Tokligence Gateway against industry standards.

## Performance Results Summary

### Tokligence Gateway vs LiteLLM (Latest: v0.3.4 PostgreSQL)

Based on [LiteLLM's official benchmarks](https://docs.litellm.ai/docs/benchmarks) running on identical GCP infrastructure:

| Metric | LiteLLM<br/>(4 instances) | Tokligence v0.3.4<br/>(PostgreSQL, 1 instance) | Improvement |
|--------|---------------------------|------------------------------------------------|-------------|
| **Throughput (RPS)** | 1,170 | **11,227** | ✅ **9.6x faster** |
| **P50 Latency** | 100 ms | **49.66 ms** | ✅ **2x faster** |
| **P95 Latency** | 150 ms | **78.63 ms** | ✅ **1.9x faster** |
| **P99 Latency** | 240 ms | **93.81 ms** | ✅ **2.6x faster** |
| **Infrastructure** | 4 instances | **1 instance** | ✅ **75% reduction** |
| **Error Rate** | Not reported | **0%** | ✅ **Perfect** |

**Peak Performance** (100 concurrent):
- **12,908 RPS** - absolute maximum throughput
- **P50: 7.75ms, P95: 16.47ms, P99: 21.15ms** - sub-100ms latencies
- **774,571 requests in 60 seconds** with 0% errors

**Cost Efficiency**:
- **38.4x better performance per dollar** than LiteLLM
- **1/4 infrastructure cost** (1 instance vs 4 instances)
- **9.6x higher throughput** with 75% fewer resources

**Test Environment**:
- **Platform**: Google Cloud Platform (GCP)
- **Gateway Server**: e2-custom-4-8192 (4 vCPUs, 8GB RAM, Intel Broadwell)
- **Load Client**: e2-custom-4-8192 (identical specs)
- **Database**: PostgreSQL with optimized connection pooling
- **Network**: Internal GCP network (< 1ms latency)

**Gateway Optimizations** (v0.3.4):
- Batch Size: 5,000 entries (previously: 100)
- Flush Interval: 200ms (previously: 1000ms)
- Buffer Size: 500,000 entries (previously: 10,000)
- Workers: 20 goroutines (previously: 1)

**Full Results**: See [Performance Comparison Report](reports/2025-11-20/performance-comparison-sqlite-vs-postgresql.md)

## Benchmark Methodology

Following [LiteLLM's benchmark approach](https://docs.litellm.ai/docs/benchmarks) with enhanced testing tools:

### Method A: Locust (LiteLLM-Compatible Testing)
- **Tool**: Locust (Python, same as LiteLLM)
- **Target**: Loopback adapter (eliminates external API latency)
- **Configuration**: Docker 4 CPU, 8GB RAM (LiteLLM-identical constraints)
- **Optimization**: Multi-process workers (4-16 processes)
- **Test Profiles**:
  - Quick: 500 users, 4 workers, 1 min (~ 2,000 RPS)
  - Full: 2,000 users, 8 workers, 5 min (~3,900 RPS)
  - Stress: 4,000 users, 16 workers, 10 min
- **Strengths**: Direct LiteLLM comparison, complex scenarios, streaming
- **Limitation**: Python GIL limits to ~4,000 RPS even with 16 workers

### Method B: Go Load Tester (Maximum Performance)
- **Tool**: Custom Go-based HTTP load generator
- **Target**: Loopback adapter
- **Configuration**: Direct testing (minimal client overhead)
- **Optimization**: Native Go concurrency (500-1000 goroutines)
- **Capability**: **7,700+ RPS** with 0% error rate
- **Strengths**: Reveals true gateway capacity, zero client bottleneck
- **Use Case**: Finding performance ceiling, capacity planning

## Quick Start

### Method A: Locust (LiteLLM-Compatible)

```bash
# Quick test (500 users, 4 workers, 1 min) - ~2,000 RPS
./scripts/benchmark/run_benchmark.sh quick

# Full benchmark (2,000 users, 8 workers, 5 min) - ~3,900 RPS
./scripts/benchmark/run_benchmark.sh full

# Stress test (4,000 users, 16 workers, 10 min)
./scripts/benchmark/run_benchmark.sh stress
```

The script automatically:
1. Creates Python virtual environment
2. Installs dependencies (locust, requests, pandas, matplotlib)
3. Builds Docker image with resource constraints (4 CPU, 8GB RAM)
4. Uses CPU pinning (CPUs 4-7) for true isolation
5. Runs Locust with multiple worker processes
6. Generates HTML reports and CSV metrics

### Method B: Go Load Tester (Maximum Performance)

```bash
# First, ensure gateway is running
docker ps | grep gateway-bench  # Check if Docker is running
# OR start local gatewayd: make gds

# Run Go load test (30s, 500 concurrent)
cd scripts/benchmark
go run loadtest.go -duration 30 -c 500

# Find maximum capacity (1000 concurrent, unlimited RPS)
go run loadtest.go -duration 60 -c 1000 -rps 0

# Options:
#   -duration N   Test duration in seconds (default: 30)
#   -c N          Concurrent workers (default: 100)
#   -rps N        Target requests/sec, 0=unlimited (default: 0)
```

### Analyze Results

```bash
# Locust: HTML report is auto-generated
open benchmark-results/benchmark-full-*-report.html

# Locust: Compare with LiteLLM benchmarks
python scripts/benchmark/analyze_results.py benchmark-results/benchmark-full-*_stats.csv

# Vegeta: View text report
cat benchmark-results/vegeta-report-*.txt

# Vegeta: View HTML plot
open benchmark-results/vegeta-plot-*.html
```

## Benchmark Scenarios

### 1. Baseline Performance (Loopback)

Tests gateway overhead with loopback adapter (no external API calls):

- **Endpoint**: `/v1/chat/completions`
- **Model**: `loopback`
- **Payload**: Small (100 tokens prompt)
- **Expected**: >1000 RPS, <150ms P95 latency

### 2. Translation Performance

Tests OpenAI ↔ Anthropic translation overhead:

- **Endpoint**: `/v1/responses` with Anthropic translation
- **Model**: `claude-3-5-sonnet`
- **Payload**: Medium (500 tokens with tool calls)
- **Expected**: >800 RPS, <200ms P95 latency

### 3. Rate Limiting Impact

Tests performance with rate limiting enabled:

- **Config**: 10,000 RPS per user, 20,000 burst
- **Expected**: <5ms overhead, no throughput degradation

### 4. Metrics Collection Impact

Tests Prometheus metrics collection overhead:

- **Config**: All metrics enabled
- **Expected**: <1ms overhead

## Target Performance

Based on LiteLLM benchmarks (4 CPU, 8GB RAM, single instance):

| Metric | LiteLLM (4 instances) | Tokligence Target (1 instance) |
|--------|----------------------|-------------------------------|
| **RPS** | 1,170 | >1,000 |
| **Median Latency** | 100 ms | <120 ms |
| **P95 Latency** | 150 ms | <180 ms |
| **P99 Latency** | 240 ms | <300 ms |
| **Gateway Overhead** | 2-8 ms | <10 ms |

*Note: Tokligence runs as a single instance vs LiteLLM's 4-instance setup*

## Results Interpretation

### Good Performance
- ✅ RPS > 1,000
- ✅ P95 latency < 180ms
- ✅ P99 latency < 300ms
- ✅ Gateway overhead < 10ms

### Needs Investigation
- ⚠️ RPS < 800
- ⚠️ P95 latency > 200ms
- ⚠️ High memory usage (>6GB)
- ⚠️ CPU throttling

### Performance Issues
- ❌ RPS < 500
- ❌ P95 latency > 300ms
- ❌ Frequent errors (>1%)
- ❌ Memory leaks

## Optimization Tips

If performance doesn't meet targets:

1. **Check Database**: SQLite vs PostgreSQL
2. **Tune Go Runtime**: GOMAXPROCS, GC settings
3. **Profile CPU/Memory**: Use pprof
4. **Review Logs**: Disable debug logging in production
5. **Check Middleware**: Ensure efficient auth/rate limiting

## Continuous Benchmarking

For CI/CD integration:

```bash
# Run quick benchmark (1 minute)
./scripts/benchmark/quick_bench.sh

# Check regression
python scripts/benchmark/compare_results.py \
  baseline.json \
  current.json \
  --threshold 10  # Fail if >10% slower
```

## Files

- `run_benchmark.sh` - Main benchmark runner
- `locustfile.py` - Locust load test definition
- `analyze_results.py` - Results analyzer and chart generator
- `docker-compose.bench.yml` - Docker setup for benchmarking
- `requirements.txt` - Python dependencies

## Method Comparison: Locust vs Vegeta

| Aspect | Locust (Method A) | Vegeta (Method C) |
|--------|-------------------|-------------------|
| **Language** | Python (gevent) | Go |
| **Client Overhead** | Moderate (GIL limitations) | Minimal (compiled, concurrent) |
| **Max RPS** | ~500-1000 RPS typical | 10,000+ RPS capable |
| **Scenarios** | Complex (streaming, tools, multiple endpoints) | Simple (single endpoint) |
| **Real-time UI** | Web UI available | CLI only |
| **Results Format** | HTML, CSV, JSON | Text, JSON, HTML plots |
| **Best For** | Realistic user behavior simulation | Finding performance ceiling |
| **LiteLLM Alignment** | ✅ Same tool used by LiteLLM | ❌ Different methodology |

**Recommendation**:
- Use **Locust** for LiteLLM comparison and scenario testing
- Use **Vegeta** to verify maximum throughput capacity

## Comparing with LiteLLM

Locust results automatically include comparison:

```bash
./scripts/benchmark/run_benchmark.sh full

# Manual analysis:
python scripts/benchmark/analyze_results.py benchmark-results/benchmark-full-*_stats.csv
```

Expected output:
```
Tokligence Gateway vs LiteLLM (4 CPU, 8GB RAM)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Metric              Tokligence    LiteLLM    Status
RPS                 1,050         1,170      ✅ Good
Median Latency      95 ms         100 ms     ✅ Good
P95 Latency         145 ms        150 ms     ✅ Good
P99 Latency         220 ms        240 ms     ✅ Good
Gateway Overhead    6 ms          2-8 ms     ✅ Good
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Note: LiteLLM used 4 instances, Tokligence uses 1 instance
```

## Resource Isolation

The benchmark script automatically configures Docker for optimal resource isolation:

### CPU Isolation
- **8+ cores**: Uses `--cpuset-cpus=4-7` to dedicate CPUs 4-7 exclusively to the container
  - CPUs 0-3 remain for system and other processes
  - True CPU isolation with no interference
- **< 8 cores**: Falls back to `--cpus=4` quota-based limiting
  - Shares CPU time but limits to 4 CPU worth of compute

### Memory Isolation
- `--memory=8g`: Hard limit at 8GB
- `--memory-swap=8g`: Same as memory (no additional swap)
- `--memory-swappiness=0`: Disable swapping for consistent performance

### Verification

Check resource allocation:
```bash
# View container resource usage
docker stats gateway-bench

# Verify CPU pinning
docker inspect gateway-bench | grep -A 5 CpusetCpus

# Check memory limits
docker inspect gateway-bench | grep -E "Memory|Swap"
```

## Advanced Testing

### Stress Test

Push gateway to its limits:

```bash
./scripts/benchmark/stress_test.sh
```

### Endurance Test

Run for 24 hours to detect memory leaks:

```bash
./scripts/benchmark/endurance_test.sh
```

### Spike Test

Test burst handling:

```bash
./scripts/benchmark/spike_test.sh
```

## Troubleshooting

**Q: Locust shows connection errors**
A: Check if gateway is running: `curl http://localhost:8081/health`

**Q: Low RPS despite good latency**
A: Increase Locust workers: `--workers=4`

**Q: Results vary significantly between runs**
A: Warm up the system, run multiple times, average results

**Q: Docker resource limits not working**
A: Check: `docker stats gateway-bench`

## References

- LiteLLM Benchmarks: https://docs.litellm.ai/docs/benchmarks
- Locust Documentation: https://docs.locust.io/
- Go Performance: https://go.dev/doc/diagnostics
