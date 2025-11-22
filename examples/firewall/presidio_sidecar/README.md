# Presidio Prompt Firewall Sidecar

A Python-based sidecar service that integrates Microsoft Presidio to provide PII detection and anonymization for Tokligence Gateway.

## Overview

**Presidio is an optional component**, not required:
- ✅ **Built-in filters are sufficient** - Regex detection works for most scenarios
- ✅ **Isolated installation** - Uses virtual environment, doesn't pollute global Python
- ✅ **Enable on demand** - Only use when high accuracy is needed

## Features

- **PII Detection**: Detects 15+ PII types (email, phone, SSN, credit card, IP, etc.)
- **Anonymization**: Automatic redaction or masking
- **REST API**: Compatible with Tokligence Gateway's HTTP filter
- **Low Latency**: Most requests <200ms
- **Configurable**: Easy to customize entity types and thresholds

## Quick Start

### Option 1: Using Scripts (Recommended)

```bash
# One-command install (creates venv + installs dependencies)
./setup.sh

# Start service
./start.sh

# Stop service
./stop.sh
```

### Option 2: Manual Installation

```bash
# Create virtual environment (isolated dependencies)
python3 -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Download spaCy language model (required by Presidio)
python -m spacy download en_core_web_lg
```

### 2. Run the Service

```bash
python main.py
```

The service will start on `http://localhost:7317`

### 3. Test the Service

```bash
# Test PII detection
curl -X POST http://localhost:7317/v1/filter/input \
  -H "Content-Type: application/json" \
  -d '{
    "input": "My email is john.doe@example.com and my SSN is 123-45-6789"
  }'
```

Expected response:
```json
{
  "allowed": false,
  "block": true,
  "block_reason": "Critical PII detected: US_SSN",
  "redacted_input": "My email is [EMAIL] and my SSN is [SSN]",
  "detections": [
    {
      "filter_name": "presidio",
      "type": "pii",
      "severity": "medium",
      "message": "Detected EMAIL_ADDRESS in input",
      "location": "input",
      "details": {...}
    },
    {
      "filter_name": "presidio",
      "type": "pii",
      "severity": "critical",
      "message": "Detected US_SSN in input",
      "location": "input",
      "details": {...}
    }
  ],
  "entities": [...],
  "annotations": {
    "pii_count": 2,
    "pii_types": ["EMAIL_ADDRESS", "US_SSN"]
  }
}
```

## API Endpoints

### POST /v1/filter/input
Filter and analyze input text (request to LLM)

### POST /v1/filter/output
Filter and analyze output text (response from LLM)

### POST /v1/filter
Filter both input and output

### GET /health
Health check endpoint

## Supported PII Types

- **Critical Severity**:
  - CREDIT_CARD
  - US_SSN
  - US_PASSPORT

- **High Severity**:
  - CRYPTO (cryptocurrency addresses)

- **Medium Severity**:
  - EMAIL_ADDRESS
  - PHONE_NUMBER

- **Low Severity**:
  - PERSON (names)
  - LOCATION
  - IP_ADDRESS

## Integration with Tokligence Gateway

Add this to your `firewall.ini`:

```ini
[prompt_firewall]
enabled = true
mode = enforce

[firewall_input_filters]
filter_presidio_enabled = true
filter_presidio_priority = 5
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow

[firewall_output_filters]
filter_presidio_enabled = true
filter_presidio_priority = 5
filter_presidio_endpoint = http://localhost:7317/v1/filter/output
filter_presidio_timeout_ms = 500
filter_presidio_on_error = bypass
```

## Configuration

Edit `main.py` to customize:

- `PII_ENTITIES`: List of entity types to detect
- `ENTITY_MASKS`: Custom redaction masks
- `SEVERITY_MAP`: Severity levels for blocking decisions
- Port and host in `uvicorn.run()`

## Performance Considerations

- **Cold Start**: First request takes ~2-3 seconds (model loading)
- **Subsequent Requests**: ~50-200ms depending on text length
- **Memory**: ~500MB-1GB RAM
- **Scalability**: Can handle ~100-200 req/s on a single instance
- **Optimization**: For high-throughput scenarios:
  - Run multiple instances behind a load balancer
  - Use connection pooling
  - Consider caching results for identical inputs

## Docker Deployment

```bash
# Build image
docker build -t presidio-firewall .

# Run container
docker run -p 7317:7317 presidio-firewall
```

## Troubleshooting

### "Model 'en_core_web_lg' not found"
```bash
python -m spacy download en_core_web_lg
```

### High memory usage
Reduce model size by using `en_core_web_sm`:
```bash
python -m spacy download en_core_web_sm
```
Then modify `main.py` to use the smaller model.

### Slow performance
- Ensure the service is not restarting on each request
- Check that models are loaded once at startup
- Consider using a faster machine or GPU acceleration

## License

MIT
