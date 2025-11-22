# Prompt Firewall Quick Reference

## 30-Second Start

```bash
# 1. Copy config
cp examples/firewall/configs/firewall.ini config/

# 2. Edit mode (monitor or enforce)
# mode: monitor  # Safe for testing
# mode: enforce  # Active protection

# 3. Start gateway
make gds

# 4. Check it works
tail -f logs/gatewayd.log | grep firewall
```

## Common Configs

### Just Monitor (Safe)
```ini
[prompt_firewall]
enabled = true
mode = monitor

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

### Enforce & Redact
```ini
[prompt_firewall]
enabled = true
mode = enforce

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

### With Presidio
```ini
[prompt_firewall]
enabled = true
mode = enforce

[firewall_input_filters]
filter_presidio_enabled = true
filter_presidio_priority = 10
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow

[firewall_output_filters]
filter_presidio_enabled = true
filter_presidio_priority = 10
filter_presidio_endpoint = http://localhost:7317/v1/filter/output
filter_presidio_timeout_ms = 500
filter_presidio_on_error = bypass
```

## PII Types

| Code | Detects | Example |
|------|---------|---------|
| EMAIL | Email addresses | user@example.com |
| PHONE | Phone numbers | 555-123-4567 |
| SSN | Social Security | 123-45-6789 |
| CREDIT_CARD | Credit cards | 4111-1111-1111-1111 |
| IP_ADDRESS | IP addresses | 192.168.1.1 |
| API_KEY | API keys | sk-xxx...xxx |

## Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| **monitor** | Log only, don't block | Testing, tuning |
| **enforce** | Block violations | Production |
| **disabled** | Turn off firewall | Bypass |

## Error Actions

```ini
[firewall_input_filters]
filter_presidio_on_error = allow   # Continue on service error
# filter_presidio_on_error = block   # Block on service error (fail-closed)
# filter_presidio_on_error = bypass  # Skip this filter on error
```

## Performance

| Filter | Latency | Accuracy |
|--------|---------|----------|
| Built-in regex | ~10ms | 85% |
| Presidio | ~100ms | 95% |
| Hybrid | ~50ms | 95% |

## Test Commands

```bash
# Test with PII
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Email: test@example.com"}]}'

# Check logs
grep firewall logs/gatewayd.log

# Test Presidio
curl http://localhost:7317/health
```

## Troubleshooting

**Not working?**
```bash
# Check if enabled
grep "firewall configured" logs/gatewayd.log

# Enable debug
log_level=debug  # in config/setting.ini
```

**Too slow?**
```ini
# Reduce timeout
[firewall_input_filters]
filter_presidio_timeout_ms = 300  # default is 500

# Or disable slow filters
filter_presidio_enabled = false
```

**Too many false positives?**
```ini
# Note: Current implementation detects all PII types
# Future versions will support selective type filtering
[firewall_input_filters]
filter_pii_regex_enabled = true
```

## Files

```
examples/firewall/
├── configs/firewall.ini           # Start here
├── presidio_sidecar/main.py        # Python service
└── README.md                       # Full guide

docs/PROMPT_FIREWALL.md             # Complete docs
```

## Next Steps

1. Start with `mode: monitor`
2. Review logs for 1-2 weeks
3. Tune `enabled_types`
4. Switch to `mode: enforce`
5. Add Presidio for accuracy

Full docs: `docs/PROMPT_FIREWALL.md`
