# Presidio PII Detection Sidecar

A Python-based sidecar service using [Microsoft Presidio](https://microsoft.github.io/presidio/) for advanced PII detection and anonymization. Supports **multilingual detection** including English and Chinese.

## Overview

| Feature | Built-in (Regex) | Presidio Sidecar |
|---------|-----------------|------------------|
| **Accuracy** | Good for structured data | Better for unstructured text |
| **Languages** | Pattern-based (any) | EN + ZH (extensible) |
| **Person Names** | ❌ Not supported | ✅ NLP-based detection |
| **Memory** | ~0 MB | ~600 MB |
| **Latency** | <1ms | ~10-30ms |
| **Dependencies** | None | Python + spaCy |

**When to use Presidio:**
- Need to detect person names in free text
- Processing multilingual content (EN + ZH)
- Higher accuracy requirements for unstructured data

**When Built-in is sufficient:**
- Detecting structured PII (emails, phones, SSN, credit cards)
- Low latency requirements (<1ms)
- Minimal deployment footprint

## Quick Start

```bash
# From project root
make pii-setup    # Install dependencies + download models (~600MB)
make pii-start    # Start Presidio sidecar on :7317
make pii-test     # Run multilingual PII detection tests
make pii-stop     # Stop Presidio sidecar
make pii-status   # Check service status
```

## Supported Languages & Models

### Default: English + Chinese

| Language | Model | Size | Download Command |
|----------|-------|------|------------------|
| English | `en_core_web_lg` | ~600MB | Auto-downloaded by setup |
| Chinese | Custom Regex | 0 MB | Built-in (no download needed) |

### Chinese PII Detection (中文PII检测)

Chinese detection is implemented via **regex patterns** (no NLP model required):

| Entity Type | Example | Detection Method |
|-------------|---------|------------------|
| 人名 (PERSON) | 张三, 李明华, 欧阳明月 | Surname + given name pattern |
| 手机号 (PHONE) | 13800138000 | 11-digit mobile pattern |
| 身份证 (CN_ID_CARD) | 110101199001011234 | 18-digit ID with date validation |

**Supported Surnames (百家姓):** 100+ common Chinese surnames including:
- Single-char: 王李张刘陈杨黄赵吴周徐孙马朱胡郭何林高罗...
- Double-char: 欧阳, 上官, 司马, 诸葛, 皇甫, 令狐...

### Adding More Languages

To add support for additional languages (e.g., German, French, Japanese):

```bash
# 1. Download the spaCy model
source venv/bin/activate
python -m spacy download de_core_news_sm  # German (~15MB)
python -m spacy download fr_core_news_sm  # French (~15MB)
python -m spacy download ja_core_news_sm  # Japanese (~40MB)

# 2. Register the model in main.py (see code comments)
```

**Available spaCy Models:**

| Language | Model | Size | Entities |
|----------|-------|------|----------|
| German | `de_core_news_sm` | 15MB | PER, LOC, ORG |
| French | `fr_core_news_sm` | 15MB | PER, LOC, ORG |
| Spanish | `es_core_news_sm` | 12MB | PER, LOC, ORG |
| Japanese | `ja_core_news_sm` | 40MB | PERSON, GPE, ORG |
| Multi (100+) | `xx_ent_wiki_sm` | 50MB | PER, LOC, ORG |

Full list: https://spacy.io/models

## Performance Characteristics

### Latency Benchmarks

| Text Length | Latency | Entities Detected |
|-------------|---------|-------------------|
| Short (~50 chars) | 10-15ms | 1-2 |
| Medium (~200 chars) | 15-25ms | 3-5 |
| Long (~500 chars) | 25-40ms | 5-10 |

### Memory Usage

| Component | Memory |
|-----------|--------|
| Base Python + FastAPI | ~100MB |
| spaCy `en_core_web_lg` | ~500MB |
| Custom Chinese recognizers | ~10MB |
| **Total** | **~600MB** |

### Scaling Considerations

| Scenario | Recommendation |
|----------|----------------|
| Low traffic (<100 req/s) | Single instance |
| Medium traffic (100-500 req/s) | Multiple workers (`PRESIDIO_WORKERS=4`) |
| High traffic (>500 req/s) | Multiple instances behind load balancer |

## Configuration

### Environment Variables

```bash
# Server
PRESIDIO_HOST=0.0.0.0        # Bind address
PRESIDIO_PORT=7317           # Port (default: 7317)
PRESIDIO_WORKERS=1           # Number of worker processes
PRESIDIO_LOG_LEVEL=info      # Logging level

# Features
PRESIDIO_ENABLE_CHINESE=true # Enable Chinese PII detection
```

### Integration with Gateway

Add to `config/firewall.ini`:

```ini
[firewall_input_filters]
filter_presidio_enabled = true
filter_presidio_priority = 20
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow

[firewall_output_filters]
filter_presidio_enabled = true
filter_presidio_priority = 20
filter_presidio_endpoint = http://localhost:7317/v1/filter/output
filter_presidio_timeout_ms = 500
filter_presidio_on_error = allow
```

## API Reference

### POST /v1/filter/input

Filter input text for PII.

```bash
curl -X POST http://localhost:7317/v1/filter/input \
  -H "Content-Type: application/json" \
  -d '{"input": "My name is John Smith, email john@example.com"}'
```

Response:
```json
{
  "allowed": true,
  "block": false,
  "redacted_input": "My name is [PERSON], email [EMAIL]",
  "detections": [...],
  "entities": [...],
  "annotations": {
    "pii_count": 2,
    "pii_types": ["PERSON", "EMAIL_ADDRESS"],
    "processing_time_ms": 15
  }
}
```

### POST /v1/filter/output

Filter LLM output for PII leakage.

### GET /health

Health check endpoint.

```json
{
  "status": "healthy",
  "analyzer_ready": true,
  "anonymizer_ready": true,
  "supported_entities": ["PERSON", "EMAIL_ADDRESS", ...]
}
```

## Supported Entity Types

### Critical Severity (blocks request)
- `CREDIT_CARD` - Credit card numbers
- `US_SSN` - US Social Security Number
- `US_PASSPORT` - US Passport number
- `CN_ID_CARD` - Chinese ID card (身份证)

### High Severity
- `CRYPTO` - Cryptocurrency addresses

### Medium Severity
- `EMAIL_ADDRESS` - Email addresses
- `PHONE_NUMBER` - Phone numbers (US, CN, intl)

### Low Severity
- `PERSON` - Person names (EN, ZH)
- `LOCATION` - Location/addresses
- `IP_ADDRESS` - IP addresses

## Docker Deployment

```bash
# Build
docker build -t presidio-firewall .

# Run
docker run -d \
  -p 7317:7317 \
  -e PRESIDIO_WORKERS=4 \
  --name presidio \
  presidio-firewall
```

## Troubleshooting

### "Model 'en_core_web_lg' not found"
```bash
source venv/bin/activate
python -m spacy download en_core_web_lg
```

### High memory usage
Use a smaller model:
```bash
python -m spacy download en_core_web_sm  # 12MB instead of 600MB
# Edit main.py to use en_core_web_sm
```

### Slow first request
This is normal - models are loaded on first request. Subsequent requests are fast.

### Chinese names not detected
Ensure `PRESIDIO_ENABLE_CHINESE=true` (default).

## Testing

```bash
# Unit tests (Presidio sidecar only)
make pii-test

# End-to-end tests (Gateway + Presidio)
./tests/firewall/test_e2e_pii_firewall.sh --setup
```

## License

MIT
