# Tokligence Gateway Prompt Firewall Examples

This directory contains examples and configurations for the Prompt Firewall feature in Tokligence Gateway.

## Directory Structure

```
examples/firewall/
├── README.md                    # This file
├── configs/                     # Configuration examples
│   ├── firewall.ini           # Basic configuration
│   ├── firewall-enforce.ini   # Strict enforcement mode
│   └── firewall-monitor-only.ini  # Monitor-only mode
└── presidio_sidecar/           # Python Presidio integration
    ├── main.py                 # FastAPI service
    ├── requirements.txt        # Python dependencies
    ├── Dockerfile              # Container image
    └── README.md               # Detailed guide
```

## Quick Start

### Option 1: Built-in PII Regex Only

Fastest and simplest option. Uses Go-based regex patterns for PII detection.

**Latency**: ~5-10ms per request

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact  # redact (recommended) | monitor | enforce | disabled

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**Test it**:
```bash
# Start gateway
make gds

# Send test request with PII
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "My email is test@example.com"}
    ]
  }'

# Check logs
tail -f logs/gatewayd.log | grep firewall
```

### Option 2: With Presidio Sidecar

Most accurate option. Uses Microsoft Presidio with NLP models.

**Latency**: ~50-200ms per request

**Setup**:
```bash
# 1. Start Presidio sidecar
cd examples/firewall/presidio_sidecar
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python -m spacy download en_core_web_lg
python main.py &  # Runs on port 7317

# 2. Configure gateway with HTTP filter
cp configs/firewall-enforce.ini config/firewall.ini

# 3. Start gateway
make gds
```

**Test it**:
```bash
# Send request with PII
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "My SSN is 123-45-6789 and email is john@example.com"}
    ]
  }'

# Should be blocked or redacted based on config
```

## Configuration Examples

### 1. Monitor Mode (Development)

Use this during development to understand your PII patterns without blocking requests.

```ini
# configs/firewall-monitor-only.ini
[prompt_firewall]
enabled = true
mode = monitor  # Log only, never block

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**When to use**:
- Initial deployment
- Testing and tuning
- Analytics and reporting

**Log output**:
```
[firewall.monitor] location=input pii_count=2 types=[EMAIL, PHONE]
```

### 2. Enforce Mode (Production)

Use this in production to actively block or redact sensitive information.

```ini
# configs/firewall-enforce.ini
[prompt_firewall]
enabled = true
mode = enforce  # Actively block violations

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**When to use**:
- Production environments
- Compliance requirements (GDPR, HIPAA)
- Critical data protection

**Response on block**:
```json
{
  "error": "request blocked by firewall: Critical PII detected: US_SSN"
}
```

### 3. Hybrid Mode (Multi-Layer)

Combine built-in regex (fast) with Presidio (accurate) for best results.

```ini
[prompt_firewall]
enabled = true
mode = enforce

# Layer 1: Fast regex pre-filter (priority 5)
[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 5

# Layer 2: Deep analysis with Presidio (priority 10)
filter_presidio_enabled = true
filter_presidio_priority = 10
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow  # Don't block if Presidio is down

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**Benefits**:
- Fast path for obvious PII (regex)
- Deep analysis for complex cases (Presidio)
- Graceful degradation if Presidio fails

## Supported PII Types

### Built-in Regex Patterns

| Type | Example | Confidence |
|------|---------|-----------|
| EMAIL | user@example.com | 95% |
| PHONE | +1-555-123-4567 | 90% |
| SSN | 123-45-6789 | 95% |
| CREDIT_CARD | 4111-1111-1111-1111 | 85% |
| IP_ADDRESS | 192.168.1.1 | 80% |
| API_KEY | sk-xxx...xxx | 75% |

### Presidio Entities

In addition to the above, Presidio detects:

- PERSON (names)
- LOCATION (addresses, cities)
- US_PASSPORT
- US_DRIVER_LICENSE
- CRYPTO (wallet addresses)
- IBAN_CODE
- MEDICAL_LICENSE
- And 40+ more entity types

## Performance Comparison

| Scenario | Latency | Accuracy | Cost |
|----------|---------|----------|------|
| **Built-in regex only** | 5-10ms | 85% | Free |
| **Presidio sidecar** | 50-200ms | 95% | $50-100/mo (compute) |
| **Hybrid (regex + Presidio)** | 10-150ms | 95% | $50-100/mo |
| **SaaS (future)** | 20-100ms | 95% | Pay per request |

## Integration Checklist

- [ ] Choose operating mode (monitor vs enforce)
- [ ] Configure PII types to detect
- [ ] Set up logging and monitoring
- [ ] (Optional) Start Presidio sidecar
- [ ] Test with sample requests
- [ ] Review logs for false positives
- [ ] Gradually increase enforcement level
- [ ] Set up alerts for blocks

## Testing

### Unit Tests

```bash
cd internal/firewall
go test -v
```

### Integration Tests

```bash
# Test built-in filters
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Test email: john@example.com, Phone: 555-1234"}
    ]
  }'

# Test Presidio sidecar
curl -X POST http://localhost:7317/v1/filter/input \
  -H "Content-Type: application/json" \
  -d '{
    "input": "My SSN is 123-45-6789"
  }'
```

### Load Testing

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test throughput
hey -n 1000 -c 10 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
  http://localhost:8081/v1/chat/completions
```

## Troubleshooting

### Firewall Not Working

1. Check if firewall is enabled:
   ```bash
   grep "firewall configured" logs/gatewayd.log
   ```

2. Verify configuration is loaded:
   ```go
   // In your bootstrap code
   if pipeline == nil {
       log.Fatal("firewall pipeline not initialized")
   }
   ```

3. Enable debug logging:
   ```ini
   # config/setting.ini
   log_level=debug
   ```

### High Latency

1. Check filter performance:
   ```bash
   grep "filter.*completed" logs/gatewayd.log
   ```

2. Reduce Presidio timeout:
   ```ini
   [firewall_input_filters]
   filter_presidio_timeout_ms = 300  # Reduce from 500ms
   ```

3. Use monitor mode temporarily:
   ```ini
   [prompt_firewall]
   mode = monitor  # Skip enforcement overhead
   ```

### False Positives

1. Review detections:
   ```bash
   grep "firewall.detection" logs/gatewayd.log | jq
   ```

2. Adjust patterns or confidence thresholds

3. Whitelist specific patterns (custom filter)

### Presidio Connection Failures

1. Check sidecar health:
   ```bash
   curl http://localhost:7317/health
   ```

2. Check sidecar logs:
   ```bash
   # If running in foreground, check terminal output
   # If in background: check process logs
   ```

3. Set graceful fallback:
   ```ini
   [firewall_input_filters]
   filter_presidio_on_error = bypass  # Don't block if sidecar is down
   ```

## Production Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  gateway:
    image: tokligence/gateway:latest
    ports:
      - "8081:8081"
    volumes:
      - ./config:/app/config
    environment:
      - TOKLIGENCE_LOG_LEVEL=info
      - TOKLIGENCE_PROMPT_FIREWALL_ENABLED=true
      - TOKLIGENCE_PROMPT_FIREWALL_MODE=redact
    depends_on:
      - presidio

  presidio:
    build: ./presidio_sidecar
    ports:
      - "7317:7317"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7317/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: firewall-config
data:
  firewall.ini: |
    [prompt_firewall]
    enabled = true
    mode = enforce

    [firewall_input_filters]
    filter_pii_regex_enabled = true
    filter_pii_regex_priority = 10

    [firewall_output_filters]
    filter_pii_regex_enabled = true
    filter_pii_regex_priority = 10

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tokligence-gateway
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: gateway
        image: tokligence/gateway:latest
        volumeMounts:
        - name: config
          mountPath: /app/config
      volumes:
      - name: config
        configMap:
          name: firewall-config

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: presidio-sidecar
spec:
  replicas: 2  # Scale for throughput
  template:
    spec:
      containers:
      - name: presidio
        image: tokligence/presidio-firewall:latest
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
```

## Next Steps

1. **Tune Configuration**: Start with monitor mode, gradually increase enforcement
2. **Add Custom Patterns**: Implement domain-specific PII patterns
3. **Set Up Monitoring**: Export metrics to Prometheus/Grafana
4. **Scale Presidio**: Run multiple instances for high throughput
5. **Compliance**: Document your firewall configuration for audits

## Additional Resources

- [Main Documentation](../../docs/PROMPT_FIREWALL.md)
- [API Reference](../../internal/firewall/)
- [Presidio Documentation](https://microsoft.github.io/presidio/)
- [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
