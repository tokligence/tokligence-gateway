# Performance Quick Start

## The Challenge

Go Gateway is fast (10K+ req/s), but Python Presidio can become a bottleneck (~150 req/s per worker).

## Quick Solutions

### Individual Users (<100 req/s)

Use built-in filters only, no Presidio needed:

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**Performance**: 10K+ req/s ✅

---

### Small Teams (100-1000 req/s)

Enable multi-process Presidio:

```bash
# 1. Install Presidio
cd examples/firewall/presidio_sidecar
./setup.sh

# 2. Set Workers (based on CPU cores)
export PRESIDIO_WORKERS=8  # For 8-core CPU

# 3. Start Presidio
./start.sh

# 4. Configure Gateway
cp examples/firewall/configs/firewall-enforce.ini config/firewall.ini

# 5. Start Gateway
make gds
```

**Performance**: ~1200 req/s ✅

---

### Enterprise (1000+ req/s)

Multiple instances + load balancing:

```bash
# Docker Compose deployment
cd examples/firewall
docker-compose -f docker-compose.high-performance.yml up -d
```

**Performance**: ~2000 req/s ✅

---

### Best Performance (10K+ req/s)

Hybrid strategy (built-in + Presidio + graceful degradation):

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact

[firewall_input_filters]
# Fast path (handles 80% of cases)
filter_pii_regex_enabled = true
filter_pii_regex_priority = 5

# Deep analysis (optional, for complex cases)
# filter_presidio_enabled = true
# filter_presidio_priority = 10
# filter_presidio_endpoint = http://localhost:7317/v1/filter/input
# filter_presidio_timeout_ms = 200  # Fast timeout
# filter_presidio_on_error = allow  # Degrade gracefully on timeout

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**Performance**: 10K+ req/s ✅

---

## Performance Comparison

| Configuration | Throughput | Use Case |
|--------------|------------|----------|
| Built-in filters only | 10K+ req/s | Individual users |
| Presidio (1 worker) | ~150 req/s | Testing |
| Presidio (4 workers) | ~600 req/s | Small teams |
| Presidio (8 workers) | ~1200 req/s | Medium teams |
| 4 instances + LB | ~2000 req/s | Enterprise |
| Hybrid strategy | 10K+ req/s | High performance needs |

---

## Quick Testing

```bash
# 1. Test built-in filters
./tests/integration/firewall/test_firewall_basic.sh

# 2. Load testing (install hey if needed)
go install github.com/rakyll/hey@latest

# Test gateway endpoint
hey -n 10000 -c 100 \
  -m POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"test@example.com"}]}' \
  http://localhost:8081/v1/chat/completions
```

---

## Configuring Worker Count

```bash
# Check CPU cores
nproc  # Linux
sysctl -n hw.ncpu  # macOS

# Set Workers = CPU cores × 2
export PRESIDIO_WORKERS=16  # For 8-core CPU × 2
cd examples/firewall/presidio_sidecar
./start.sh
```

---

## Environment Variable Override

Quick mode switching without editing config files:

```bash
# Disable firewall (maximum performance)
export TOKLIGENCE_PROMPT_FIREWALL_ENABLED=false
make gds

# Use monitor mode (observability)
export TOKLIGENCE_PROMPT_FIREWALL_MODE=monitor
make gds

# Use redact mode (recommended)
export TOKLIGENCE_PROMPT_FIREWALL_MODE=redact
make gds
```

---

**Full Documentation**: See `examples/firewall/PERFORMANCE_TUNING.md` for detailed tuning guide.
