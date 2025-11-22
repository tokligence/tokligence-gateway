# Firewall Performance Tuning Guide

## Problem Analysis

The Go Gateway has excellent concurrency performance (handles 10K+ req/s), but the Python Presidio sidecar can become a bottleneck:

- **Single-process Python**: ~100-200 req/s
- **Go Gateway**: 10,000+ req/s
- **Bottleneck**: Presidio is 50-100x slower

## Solutions

### Solution 1: Multi-process Presidio (Recommended)

Use uvicorn's multi-process mode to fully utilize multi-core CPUs.

#### Configuration

```bash
# Set environment variable
export PRESIDIO_WORKERS=8  # Adjust based on CPU cores

# Start service
cd examples/firewall/presidio_sidecar
./start.sh
```

#### Performance Improvement

| Workers | Throughput | CPU Core Utilization |
|---------|-----------|---------------------|
| 1 | ~150 req/s | 1 core |
| 4 | ~600 req/s | 4 cores |
| 8 | ~1200 req/s | 8 cores |
| 16 | ~2000 req/s | 16 cores |

**Note**: Recommended to set workers to 1-2x the number of CPU cores.

#### Auto-detect CPU Cores

```bash
# Linux
PRESIDIO_WORKERS=$(nproc) ./start.sh

# macOS
PRESIDIO_WORKERS=$(sysctl -n hw.ncpu) ./start.sh
```

### Solution 2: Multiple Instances + Load Balancing

Deploy multiple Presidio instances with a load balancer in front.

#### Docker Compose Example

```yaml
version: '3.8'

services:
  gateway:
    build: .
    ports:
      - "8081:8081"
    environment:
      - TOKLIGENCE_PROMPT_FIREWALL_ENABLED=true
      - TOKLIGENCE_PROMPT_FIREWALL_MODE=redact
      # Presidio endpoint should be configured in config/firewall.ini
    depends_on:
      - presidio-lb

  # Nginx load balancer
  presidio-lb:
    image: nginx:alpine
    ports:
      - "7317:7317"
    volumes:
      - ./presidio-nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - presidio-1
      - presidio-2
      - presidio-3
      - presidio-4

  # Presidio instances 1-4
  presidio-1:
    build: ./examples/firewall/presidio_sidecar
    environment:
      - PRESIDIO_WORKERS=2
    deploy:
      resources:
        limits:
          memory: 1G

  presidio-2:
    build: ./examples/firewall/presidio_sidecar
    environment:
      - PRESIDIO_WORKERS=2
    deploy:
      resources:
        limits:
          memory: 1G

  presidio-3:
    build: ./examples/firewall/presidio_sidecar
    environment:
      - PRESIDIO_WORKERS=2
    deploy:
      resources:
        limits:
          memory: 1G

  presidio-4:
    build: ./examples/firewall/presidio_sidecar
    environment:
      - PRESIDIO_WORKERS=2
    deploy:
      resources:
        limits:
          memory: 1G
```

#### Nginx Configuration (presidio-nginx.conf)

```nginx
events {
    worker_connections 4096;
}

http {
    upstream presidio_backend {
        least_conn;  # Least connections load balancing

        server presidio-1:7317 max_fails=3 fail_timeout=30s;
        server presidio-2:7317 max_fails=3 fail_timeout=30s;
        server presidio-3:7317 max_fails=3 fail_timeout=30s;
        server presidio-4:7317 max_fails=3 fail_timeout=30s;

        keepalive 64;  # Connection pool
    }

    server {
        listen 7317;

        location / {
            proxy_pass http://presidio_backend;
            proxy_http_version 1.1;
            proxy_set_header Connection "";
            proxy_set_header Host $host;

            # Timeout configuration
            proxy_connect_timeout 5s;
            proxy_send_timeout 10s;
            proxy_read_timeout 10s;
        }

        location /health {
            access_log off;
            proxy_pass http://presidio_backend/health;
        }
    }
}
```

**Performance**: 4 instances × 2 workers = ~1600 req/s

### Solution 3: Kubernetes Horizontal Scaling

```yaml
# presidio-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: presidio-firewall
spec:
  replicas: 4  # 4 Pods
  selector:
    matchLabels:
      app: presidio-firewall
  template:
    metadata:
      labels:
        app: presidio-firewall
    spec:
      containers:
      - name: presidio
        image: presidio-firewall:latest
        env:
        - name: PRESIDIO_WORKERS
          value: "2"  # 2 workers per Pod
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        ports:
        - containerPort: 7317
        livenessProbe:
          httpGet:
            path: /health
            port: 7317
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 7317
          initialDelaySeconds: 10
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: presidio-firewall
spec:
  selector:
    app: presidio-firewall
  ports:
  - port: 7317
    targetPort: 7317
  type: ClusterIP

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: presidio-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: presidio-firewall
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

**Performance**: Auto-scaling, can reach 5000+ req/s

### Solution 4: Hybrid Strategy (Best Practice)

Combine built-in filters with Presidio to optimize overall performance.

#### Gateway Configuration

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = enforce

# Layer 1: Fast built-in filter (5-10ms, doesn't use Presidio)
[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 5

# Layer 2: Presidio deep analysis (50-200ms, only when needed)
filter_presidio_enabled = true
filter_presidio_priority = 10
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 200  # Fast timeout
filter_presidio_on_error = allow  # Continue on timeout/failure (graceful degradation)

# Output must be redacted (using fast filter)
[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**Advantages**:
- ✅ Built-in filter handles 80% of cases (fast path)
- ✅ Presidio only handles complex scenarios
- ✅ Auto-degradation on service failure
- ✅ Overall latency <50ms

### Solution 5: Caching (For Repeated Requests)

Add a caching layer if you have many duplicate requests.

#### Redis Caching Example

```python
# Add to main.py
import redis
import hashlib

redis_client = redis.Redis(
    host=os.getenv('REDIS_HOST', 'localhost'),
    port=int(os.getenv('REDIS_PORT', '6379')),
    db=0,
    decode_responses=True
)

def cache_key(text: str) -> str:
    return f"presidio:{hashlib.md5(text.encode()).hexdigest()}"

@app.post("/v1/filter/input", response_model=FilterResponse)
async def filter_input(request: FilterRequest) -> FilterResponse:
    if not request.input:
        return FilterResponse()

    # Check cache
    key = cache_key(request.input)
    cached = redis_client.get(key)
    if cached:
        return FilterResponse.parse_raw(cached)

    # Normal processing
    results = analyze_text(request.input)
    response = FilterResponse(...)

    # Cache result (1 hour)
    redis_client.setex(key, 3600, response.json())

    return response
```

**Performance Improvement**: <5ms on cache hit

## Performance Benchmarks

### Test Environment

- CPU: 8 cores
- Memory: 16GB
- Network: Local loopback

### Test Results

| Configuration | Throughput (req/s) | P50 Latency | P99 Latency |
|--------------|-------------------|-------------|-------------|
| Single-process Presidio | 150 | 60ms | 200ms |
| 4 Workers | 600 | 65ms | 250ms |
| 8 Workers | 1200 | 70ms | 300ms |
| 4 instances + LB | 1600 | 75ms | 350ms |
| Hybrid strategy | 8000+ | 15ms | 150ms |

### Load Testing Commands

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test Presidio directly
hey -n 10000 -c 100 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"input":"My email is test@example.com"}' \
  http://localhost:7317/v1/filter/input

# Test Gateway (end-to-end)
hey -n 10000 -c 100 \
  -m POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test@example.com"}]}' \
  http://localhost:8081/v1/chat/completions
```

## Capacity Planning

### Individual Users

**Scenario**: Individual developers, low traffic (<10 req/s)

**Recommended Configuration**:
```bash
# Single instance is sufficient
PRESIDIO_WORKERS=1 ./start.sh
```

**Cost**: ~500MB RAM

### Small Teams

**Scenario**: Small teams, medium traffic (100-500 req/s)

**Recommended Configuration**:
```bash
# Multi-process
PRESIDIO_WORKERS=4 ./start.sh
```

**Cost**: ~2GB RAM

### Enterprise

**Scenario**: Large deployments, high traffic (1000+ req/s)

**Recommended Configuration**:
- Multiple instances + load balancing
- Kubernetes auto-scaling
- Hybrid strategy (built-in + Presidio)

**Cost**: 10-20GB RAM (depending on instance count)

## Optimization Tips

### 1. Adjust Worker Count

```bash
# Check CPU cores
nproc  # Linux
sysctl -n hw.ncpu  # macOS

# Set Workers = CPU cores × 2
export PRESIDIO_WORKERS=16
```

### 2. Adjust Timeout

```ini
# config/firewall.ini
[firewall_input_filters]
filter_presidio_timeout_ms = 200  # Faster timeout
filter_presidio_on_error = allow  # Continue on timeout
```

### 3. Selective Presidio Enablement

```ini
# Use Presidio only for input (built-in for output)
[firewall_input_filters]
filter_presidio_enabled = true
filter_presidio_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true  # Faster
filter_pii_regex_priority = 10
```

### 4. Use Smaller Model

```bash
# Download smaller spaCy model
python -m spacy download en_core_web_sm  # Instead of en_core_web_lg

# Trade slight accuracy for 2-3x speed improvement
```

### 5. Enable Connection Pooling

Configure connection pool and keep-alive on gateway side:

```go
// internal/firewall/http_filter.go
client: &http.Client{
    Timeout: config.Timeout,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

## Monitoring Metrics

Key metrics to monitor:

```bash
# Presidio throughput
curl http://localhost:7317/health | jq

# Gateway log analysis
grep "firewall.*completed" logs/gatewayd.log | \
  awk '{sum+=$NF; count++} END {print "Avg:", sum/count "ms"}'

# System resources
top -p $(pgrep -f presidio | head -1)
```

## Troubleshooting

### Issue: Presidio CPU at 100%

**Solution**:
```bash
# Increase workers
export PRESIDIO_WORKERS=8
./stop.sh && ./start.sh
```

### Issue: Request Timeout

**Solution**:
```ini
# Increase timeout or degrade
[firewall_input_filters]
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow
```

### Issue: Out of Memory

**Solution**:
```bash
# Use smaller model
python -m spacy download en_core_web_sm

# Or reduce workers
export PRESIDIO_WORKERS=2
```

## Summary

| Scenario | Recommended Solution | Throughput |
|----------|---------------------|------------|
| Individual users | Single instance | ~150 req/s |
| Small teams | Multi-process (4-8 workers) | ~600-1200 req/s |
| Enterprise | Multiple instances + load balancing | ~2000+ req/s |
| High performance | Hybrid strategy + K8s | ~10000+ req/s |

**Best Practice**: Use hybrid strategy (built-in filters + multi-process Presidio + graceful degradation)
