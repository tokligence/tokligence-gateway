# Tokligence Guard Integration Guide

This guide explains how to integrate Tokligence Gateway with [Tokligence Guard](https://guard.tokligence.ai) for enterprise-grade sensitive data protection.

## Overview

Tokligence Guard provides a cloud-based sensitive data scanning service that detects and protects:

- **Personal Information**: Email, phone, physical addresses
- **National IDs**: SSN, Chinese ID (身份证), passport numbers
- **Financial Data**: Credit cards, bank accounts
- **Secrets**: API keys, passwords, private keys
- **Other**: Crypto addresses, medical IDs, tax IDs

By integrating with Tokligence Guard, your Gateway can scan all LLM requests for sensitive data before they reach the AI provider.

## Architecture

```
Coding Agent (Cursor, Claude Code, etc.)
    │
    │ LLM API Request
    ▼
Tokligence Gateway (self-hosted)
    │
    │ Sensitive Data Scan Request
    │ (with API Key authentication)
    ▼
Tokligence Guard Protection Engine (cloud)
    │
    │ Scan Result (allow/block/redact)
    ▼
Gateway continues to LLM or blocks request
```

## Prerequisites

1. **Tokligence Guard Account**: Sign up at [guard.tokligence.ai](https://guard.tokligence.ai)
2. **API Key**: Generate an API key from the Guard dashboard (Settings → API Keys)
3. **Tokligence Gateway**: Version 0.4.0 or later

## Configuration

### Step 1: Edit firewall.ini

Open your Gateway's `config/firewall.ini` file:

```ini
# ==============================================================================
# Tokligence Gateway Firewall Configuration
# ==============================================================================

[prompt_firewall]
enabled = true
mode = redact    # Options: disabled, monitor, enforce, redact

# ==============================================================================
# Input Filters
# ==============================================================================
[firewall_input_filters]

# Built-in Sensitive Data regex filter (fast, local)
filter_sd_regex_enabled = true
filter_sd_regex_priority = 10

# Tokligence Guard HTTP filter (cloud-based, high accuracy)
filter_http_enabled = true
filter_http_priority = 20
filter_http_endpoint = https://us.guard.tokligence.ai:7316/v1/filter/input
filter_http_timeout_ms = 5000
filter_http_on_error = allow
filter_http_header_X-API-Key = tok_your_api_key_here

# ==============================================================================
# Output Filters (optional)
# ==============================================================================
[firewall_output_filters]

# Built-in filter for detokenization in redact mode
filter_sd_regex_enabled = true
filter_sd_regex_priority = 10
```

### Step 2: Choose Your Region

Tokligence Guard is available in multiple regions. Use the endpoint closest to you:

| Region | Endpoint |
|--------|----------|
| US | `https://us.guard.tokligence.ai:7316/v1/filter/input` |
| EU | `https://eu.guard.tokligence.ai:7316/v1/filter/input` |
| Asia | `https://asia.guard.tokligence.ai:7316/v1/filter/input` |

### Step 3: Set Your API Key

Replace `tok_your_api_key_here` with your actual API key from the Guard dashboard.

You can also use environment variables for security:

```bash
export TOKLIGENCE_GUARD_API_KEY="tok_your_api_key_here"
```

Then reference it in your startup script or use a config template.

## Firewall Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `disabled` | No scanning | Development/testing |
| `monitor` | Log detections, allow all requests | Initial deployment, auditing |
| `enforce` | Block requests with critical data | Strict security |
| `redact` | Replace sensitive data with tokens, restore in responses | **Recommended for production** |

## Configuration Options

### HTTP Filter Options

| Option | Description | Default |
|--------|-------------|---------|
| `filter_http_enabled` | Enable/disable the filter | `false` |
| `filter_http_priority` | Execution order (lower = earlier) | `20` |
| `filter_http_endpoint` | Guard API endpoint URL | - |
| `filter_http_timeout_ms` | Request timeout in milliseconds | `5000` |
| `filter_http_on_error` | Behavior on error: `allow`, `block`, `bypass` | `allow` |
| `filter_http_header_*` | Custom headers (e.g., `filter_http_header_X-API-Key`) | - |

### Error Handling

- `allow`: If Guard is unavailable, allow the request (fail-open)
- `block`: If Guard is unavailable, block the request (fail-closed)
- `bypass`: Skip this filter if Guard is unavailable

## Testing Your Integration

### 1. Start the Gateway

```bash
make gds
# or
./bin/gatewayd
```

### 2. Check Firewall Status

Look for this line in the startup logs:
```
firewall configured: mode=redact filters=3 (input=2 output=1)
```

### 3. Send a Test Request

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "My email is test@example.com and SSN is 123-45-6789"}
    ]
  }'
```

### 4. Verify Detection

Check the Gateway logs for firewall activity:
```
firewall.input: mode=redact endpoint=/v1/chat/completions
firewall.input.redacted original_size=119 redacted_size=118
```

## Billing & Usage

- API calls to Tokligence Guard are metered based on your subscription plan
- Usage is tracked per API key
- View usage statistics in the Guard dashboard

## Troubleshooting

### Filter not working

1. Verify `filter_http_enabled = true`
2. Check the endpoint URL is correct
3. Verify your API key is valid

### Timeout errors

Increase the timeout:
```ini
filter_http_timeout_ms = 10000
```

### Connection refused

1. Check your network allows outbound HTTPS to Guard endpoints
2. Verify the endpoint URL includes the correct port (`:7316`)

### API key errors

1. Ensure the API key is active in the Guard dashboard
2. Check for typos in the configuration
3. Verify the header name is exactly `X-API-Key`

## Security Best Practices

1. **Never commit API keys** to version control
2. **Use environment variables** for sensitive configuration
3. **Enable `redact` mode** in production for balanced security
4. **Monitor usage** in the Guard dashboard for anomalies
5. **Rotate API keys** periodically

## Support

- Documentation: [docs.tokligence.ai](https://docs.tokligence.ai)
- Issues: [github.com/tokligence/tokligence-gateway/issues](https://github.com/tokligence/tokligence-gateway/issues)
- Email: support@tokligence.ai
