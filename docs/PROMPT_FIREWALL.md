# Prompt Firewall

Tokligence Gateway includes a built-in **Prompt Firewall** to protect sensitive data (PII) and prevent security issues in LLM interactions. The firewall provides a flexible, multi-layered filtering system that can detect and redact sensitive information in both requests and responses.

## Features

- **Multi-Layer Filtering**: Combine built-in regex filters with external service calls
- **Dual-Mode Operation**: Monitor mode for observability, enforce mode for active blocking
- **PII Detection & Redaction**: Detect and mask emails, phone numbers, SSNs, credit cards, API keys, etc.
- **External Integration**: Call out to Python-based services (Presidio, LLM Guard, custom filters)
- **Configurable**: YAML-based configuration for easy customization
- **Low Latency**: Built-in filters add <10ms overhead
- **Extensible**: Easy to add custom filters and patterns

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Tokligence Gateway                        │
│                                                              │
│  Request → [Input Filters] → LLM Provider → [Output Filters] → Response  │
│              ↓                                    ↓          │
│         - PII Regex                          - PII Regex     │
│         - HTTP (Presidio)                    - HTTP (Presidio)│
│         - Custom Filters                     - Custom Filters│
└─────────────────────────────────────────────────────────────┘
```

### Filter Pipeline

1. **Input Filters**: Process requests before sending to LLM
   - Detect PII in user prompts
   - Block or redact sensitive information
   - Prevent prompt injection attacks

2. **Output Filters**: Process LLM responses before returning to client
   - Detect PII leakage in model outputs
   - Redact sensitive information
   - Ensure compliance with data protection policies

## Quick Start

### 1. Basic Configuration

Create `config/firewall.yaml`:

```yaml
enabled: true
mode: monitor  # or "enforce"

input_filters:
  - type: pii_regex
    name: input_pii
    priority: 10
    enabled: true
    config:
      redact_enabled: false
      enabled_types:
        - EMAIL
        - PHONE
        - SSN

output_filters:
  - type: pii_regex
    name: output_pii
    priority: 10
    enabled: true
    config:
      redact_enabled: true  # Redact in outputs
      enabled_types:
        - EMAIL
        - PHONE
        - SSN
        - API_KEY
```

### 2. Load Configuration

```go
package main

import (
    "log"
    "gopkg.in/yaml.v3"
    "github.com/tokligence/tokligence-gateway/internal/firewall"
)

func main() {
    // Load firewall config
    var config firewall.Config
    data, _ := os.ReadFile("config/firewall.yaml")
    yaml.Unmarshal(data, &config)

    // Build pipeline
    pipeline, _ := config.BuildPipeline()
    pipeline.SetLogger(log.Default())

    // Configure server
    server := httpserver.New(...)
    server.SetFirewallPipeline(pipeline)
}
```

### 3. Run with Monitoring

```bash
# Start gateway with firewall in monitor mode
make gds

# Check logs for detections
tail -f logs/gatewayd.log | grep firewall
```

## Filter Types

### 1. Built-in PII Regex Filter

Fast, low-latency regex-based PII detection.

**Supported Types**:
- `EMAIL`: Email addresses
- `PHONE`: US/International phone numbers
- `SSN`: Social Security Numbers
- `CREDIT_CARD`: Credit card numbers
- `IP_ADDRESS`: IPv4 addresses
- `API_KEY`: API keys and tokens

**Configuration**:
```yaml
- type: pii_regex
  name: my_pii_filter
  priority: 10
  enabled: true
  config:
    redact_enabled: true
    enabled_types:
      - EMAIL
      - PHONE
      - SSN
```

**Performance**: ~5-10ms per request

### 2. HTTP Filter (External Services)

Call external HTTP services for advanced filtering.

**Configuration**:
```yaml
- type: http
  name: presidio_filter
  priority: 20
  enabled: true
  config:
    endpoint: http://localhost:8090/v1/filter/input
    timeout_ms: 500
    on_error: allow  # allow|block|bypass
    headers:
      X-API-Key: secret
```

**Error Handling**:
- `allow`: Continue on service error
- `block`: Block request on service error (fail-closed)
- `bypass`: Skip this filter on error

**Performance**: Depends on service (~50-200ms for Presidio)

### 3. Custom Filters

Implement the `Filter` interface:

```go
type MyFilter struct {}

func (f *MyFilter) Name() string { return "my_filter" }
func (f *MyFilter) Priority() int { return 10 }
func (f *MyFilter) Direction() firewall.Direction { return firewall.DirectionInput }

func (f *MyFilter) ApplyInput(ctx *firewall.FilterContext) error {
    // Your filtering logic
    if strings.Contains(string(ctx.RequestBody), "bad_word") {
        ctx.Block = true
        ctx.BlockReason = "Inappropriate content"
    }
    return nil
}
```

## Integration with Presidio

Microsoft Presidio provides advanced PII detection with NLP models.

### 1. Start Presidio Sidecar

```bash
cd examples/firewall/presidio_sidecar

# Install dependencies
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python -m spacy download en_core_web_lg

# Run service
python main.py
# Service runs on http://localhost:8090
```

### 2. Configure Gateway

```yaml
input_filters:
  - type: http
    name: presidio_input
    priority: 20
    enabled: true
    config:
      endpoint: http://localhost:8090/v1/filter/input
      timeout_ms: 500
      on_error: allow
```

### 3. Test

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "My email is john@example.com and SSN is 123-45-6789"}
    ]
  }'
```

Check logs for PII detections and redactions.

## Operating Modes

### Monitor Mode

Logs violations without blocking requests.

**Use Cases**:
- Initial deployment and testing
- Observability and analytics
- Understanding PII patterns in your traffic

**Example**:
```yaml
mode: monitor
policies:
  redact_pii: false  # Don't modify content
```

**Log Output**:
```
[firewall.monitor] location=input pii_count=2 types=[EMAIL, PHONE]
```

### Enforce Mode

Actively blocks requests that violate policies.

**Use Cases**:
- Production environments with strict compliance requirements
- Protecting critical PII (SSN, credit cards)
- Preventing data exfiltration

**Example**:
```yaml
mode: enforce
policies:
  redact_pii: true
  max_pii_entities: 3
  block_on_categories:
    - CRITICAL_PII
```

**Response on Block**:
```json
{
  "error": "request blocked by firewall: Critical PII detected: US_SSN"
}
```

## Configuration Examples

### Example 1: Basic Monitoring

```yaml
# Monitor all traffic, no blocking
enabled: true
mode: monitor

input_filters:
  - type: pii_regex
    name: input_monitor
    enabled: true
    config:
      redact_enabled: false

output_filters:
  - type: pii_regex
    name: output_monitor
    enabled: true
    config:
      redact_enabled: false
```

### Example 2: Enforce with Redaction

```yaml
# Block critical PII, redact everything else
enabled: true
mode: enforce

input_filters:
  - type: pii_regex
    name: input_strict
    enabled: true
    config:
      redact_enabled: true

output_filters:
  - type: pii_regex
    name: output_strict
    enabled: true
    config:
      redact_enabled: true

policies:
  max_pii_entities: 2  # Block if >2 PII found
```

### Example 3: Multi-Layer with Presidio

```yaml
# Built-in + Presidio for comprehensive protection
enabled: true
mode: enforce

input_filters:
  # Fast regex pre-filter
  - type: pii_regex
    name: quick_filter
    priority: 5
    enabled: true
    config:
      redact_enabled: false

  # Deep analysis with Presidio
  - type: http
    name: presidio_deep
    priority: 10
    enabled: true
    config:
      endpoint: http://localhost:8090/v1/filter/input
      timeout_ms: 1000
      on_error: allow
```

## Performance Considerations

### Latency Impact

| Filter Type | Latency | Throughput |
|------------|---------|------------|
| PII Regex (built-in) | ~5-10ms | 10K+ req/s |
| HTTP (Presidio) | ~50-200ms | 100-200 req/s per instance |
| Custom Go Filter | ~1-5ms | Varies |

### Optimization Tips

1. **Use Built-in Filters First**: Place fast regex filters at lower priority numbers
2. **Cache HTTP Responses**: Implement caching for identical inputs
3. **Scale Sidecars**: Run multiple Presidio instances behind load balancer
4. **Set Timeouts**: Use aggressive timeouts (300-500ms) for external services
5. **Monitor Mode First**: Start with monitoring before enforcing

### High-Throughput Setup

```yaml
input_filters:
  # Priority 5: Fast regex (always runs)
  - type: pii_regex
    priority: 5
    enabled: true

  # Priority 10: Presidio (only for non-cached)
  - type: http
    priority: 10
    enabled: true
    config:
      timeout_ms: 300  # Aggressive timeout
      on_error: bypass  # Don't block on timeout
```

## Security Best Practices

1. **Defense in Depth**: Use multiple filter layers
2. **Fail-Closed for Critical**: Set `on_error: block` for critical PII filters
3. **Redact Outputs**: Always enable `redact_enabled: true` for output filters
4. **Monitor First**: Test in monitor mode before enforcing
5. **Regular Updates**: Keep PII patterns and Presidio models updated
6. **Log Review**: Regularly review firewall logs for false positives/negatives

## Monitoring & Observability

### Log Format

```
[firewall] input filter presidio_input completed in 87ms
[firewall.monitor] location=input pii_count=2 types=[EMAIL, SSN]
[firewall.debug] location=input type=pii severity=critical details={...}
[firewall.input.blocked] endpoint=/v1/chat/completions model=gpt-4 reason=Critical PII detected
```

### Metrics to Track

- Detection count by type (email, SSN, etc.)
- Block rate (enforced vs allowed)
- Latency per filter
- False positive rate
- Service availability (for HTTP filters)

### Integration with Observability Tools

Export firewall metrics to your monitoring stack:

```go
// Example: Export to Prometheus
firewallDetections := prometheus.NewCounterVec(
    prometheus.CounterOpts{Name: "firewall_detections_total"},
    []string{"type", "location", "severity"},
)
```

## Troubleshooting

### Issue: High Latency

**Symptoms**: Requests taking >1s
**Solutions**:
- Check HTTP filter timeouts
- Scale Presidio sidecar instances
- Disable slow filters temporarily
- Use built-in regex filters only

### Issue: False Positives

**Symptoms**: Legitimate requests being blocked
**Solutions**:
- Review detection patterns
- Adjust confidence thresholds in Presidio
- Use monitor mode to collect data
- Whitelist specific patterns

### Issue: PII Not Detected

**Symptoms**: Known PII passing through
**Solutions**:
- Check enabled_types configuration
- Verify filter priority order
- Test patterns in isolation
- Enable debug logging

### Issue: Sidecar Connection Failures

**Symptoms**: `on_error` triggers frequently
**Solutions**:
- Check sidecar health: `curl http://localhost:8090/health`
- Verify network connectivity
- Check sidecar logs
- Increase timeout_ms
- Set `on_error: bypass` temporarily

## Future Enhancements

Planned features for future releases:

- **Multi-Modal Support**: Image and audio PII detection
- **RAG Firewall**: Filter retrieved documents for sensitive content
- **Tool Call Firewall**: Validate MCP/Agent tool call parameters
- **Prompt Injection Detection**: Detect and block jailbreak attempts
- **Content Safety**: Integration with Llama Guard 3, NeMo Guardrails
- **Policy Engine**: Advanced rule engine with CEL expressions
- **Compliance Reports**: Automated compliance reporting (GDPR, HIPAA)

## API Reference

See [firewall package documentation](../internal/firewall/) for detailed API reference.

## Examples

See [examples/firewall/](../examples/firewall/) for complete working examples:

- `configs/`: Sample YAML configurations
- `presidio_sidecar/`: Python Presidio integration
- `tests/`: Integration test scripts

## Support

For questions and issues:
- GitHub Issues: https://github.com/tokligence/tokligence-gateway/issues
- Documentation: https://github.com/tokligence/tokligence-gateway/docs
- Community: [Join our discussions](https://github.com/tokligence/tokligence-gateway/discussions)
