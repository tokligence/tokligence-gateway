# Presidio PII Detection Sidecar

A Python-based sidecar service using [Microsoft Presidio](https://microsoft.github.io/presidio/) for advanced PII detection and anonymization. Supports **multilingual detection** including English and Chinese.

> **Note:** This is a **reference implementation** demonstrating how to integrate Presidio with Tokligence Gateway. Detection accuracy depends on the NLP models used and may require tuning for production use cases.

## Overview

| Feature | Built-in (Regex) | Presidio (spaCy) | Presidio (XLM-RoBERTa) |
|---------|-----------------|------------------|------------------------|
| **Accuracy** | Good for structured data | Good for text | **Best for text** |
| **Languages** | Pattern-based (any) | 100+ (multilingual NLP) | 10+ (transformer) |
| **Person Names** | ❌ Not supported | ✅ NLP-based | ✅ **Best boundary detection** |
| **Chinese Names** | Pattern-based | ⚠️ Limited | ✅ **Accurate boundaries** |
| **Memory** | ~0 MB | ~150 MB | ~1.5 GB |
| **Latency (CPU)** | <1ms | ~3-15ms | **~40-80ms** |
| **Dependencies** | None | Python + spaCy | Python + transformers |

**When to use Presidio:**
- Need to detect person names in free text
- Processing multilingual content
- Higher accuracy requirements for unstructured data

**When to use XLM-RoBERTa (recommended for Chinese):**
- Need **accurate entity boundary detection** in Chinese text
- Same name appearing multiple times should get the **same unique token**
- Example: "张三今天来了，张三说他很高兴" → "张三" correctly detected (not "张三今")

**When Built-in is sufficient:**
- Detecting structured PII (emails, phones, SSN, credit cards)
- Low latency requirements (<1ms)
- Minimal deployment footprint

## Quick Start

```bash
# From project root
make pii-setup    # Install dependencies + download models (~600MB)
make pii-start    # Start Presidio sidecar on :7317 (spaCy mode)
make pii-test     # Run multilingual PII detection tests
make pii-stop     # Stop Presidio sidecar
make pii-status   # Check service status

# XLM-RoBERTa mode (recommended for Chinese)
cd examples/firewall/presidio_sidecar
pip install transformers torch protobuf sentencepiece
PRESIDIO_NER_ENGINE=xlmr python main.py

# Model management
make pii-model-sm   # Download small model (12MB, fast, lower accuracy)
make pii-model-lg   # Download large model (600MB, good accuracy, default)
make pii-model-trf  # Download transformer model (best accuracy, needs GPU)
make pii-start-trf  # Start with transformer model
```

## NER Engine Selection

### XLM-RoBERTa (Recommended for Chinese)

XLM-RoBERTa provides the most accurate entity boundary detection, especially for Chinese text where there are no word separators.

```bash
# Enable XLM-RoBERTa
export PRESIDIO_NER_ENGINE=xlmr
export XLMR_NER_MODEL=hrl        # Options: hrl (10 langs), wikiann (20 langs)
export XLMR_NER_DEVICE=-1        # -1=CPU, 0+=GPU

# Start with XLM-RoBERTa
python main.py
```

**Performance on CPU:**
```
Short text (~20 chars):  40-50ms
Medium text (~100 chars): 60-80ms
Long text (~200 chars):   70-90ms
```

**Key advantage - Correct Chinese name boundaries:**
```
Input:  "张三今天来了，张三说他很高兴，李四也来了"
Output: "[PERSON_25c0fe]今天来了，[PERSON_25c0fe]说他很高兴，[PERSON_f932aa]也来了"

✅ Same name (张三) → Same token ([PERSON_25c0fe])
✅ Different name (李四) → Different token ([PERSON_f932aa])
✅ Correct boundaries: "张三" not "张三今"
```

### spaCy (Default, Faster)

spaCy mode uses traditional NLP models. Faster but less accurate for Chinese name boundaries.

```bash
# Default mode (spaCy)
export PRESIDIO_NER_ENGINE=spacy
export PRESIDIO_SPACY_MODEL=xx_ent_wiki_sm  # Multilingual

python main.py
```

## Supported Languages & Models

### Important: Accuracy Depends on Model Choice

Detection accuracy varies significantly based on the NLP model used:

| Model | Size | Languages | Latency | Use Case |
|-------|------|-----------|---------|----------|
| `xx_ent_wiki_sm` | 11MB | **100+** | ~3-5ms | **Multilingual (default)** |
| `en_core_web_sm` | 12MB | EN only | ~5ms | Development/testing |
| `en_core_web_lg` | 600MB | EN only | ~15ms | High-accuracy English |
| `en_core_web_trf` | 450MB + PyTorch | EN only | ~50-100ms (GPU) / ~1-2s (CPU) | Best English + GPU |

### Default: Multilingual Mode (xx_ent_wiki_sm)

The default configuration uses **`xx_ent_wiki_sm`** multilingual model, which:
- Supports **100+ languages** including English, Chinese, German, French, Spanish, Japanese, Russian, etc.
- Is lightweight (~11MB) and fast (~3-5ms per request)
- Combined with custom Chinese recognizers for better accuracy

### Test Results Comparison

#### Multilingual Model (`xx_ent_wiki_sm`) - Default

Tests run with `xx_ent_wiki_sm` + custom Chinese recognizers (29/29 passed):

| Language | Person Detection | Other PII | Latency |
|----------|-----------------|-----------|---------|
| 🇺🇸 English | ✅ John Smith | Email, IP, CC | 3-5ms |
| 🇨🇳 Chinese | ✅ 张三, 李明华, 欧阳明月 | Phone, ID Card | 3-4ms |
| 🇩🇪 German | ✅ Hans Müller, Berlin | - | 3-5ms |
| 🇫🇷 French | ✅ Pierre Dupont | - | 3-4ms |
| 🇪🇸 Spanish | ✅ Carlos García | - | 4-5ms |
| 🇯🇵 Japanese | ✅ 山田太郎 | - | 2-4ms |
| 🇷🇺 Russian | ✅ Иван Петров | - | 3-4ms |
| 🇰🇷 Korean | ⚠️ Limited | - | N/A |

**Note:** Korean name detection is limited in `xx_ent_wiki_sm`. For Korean support, consider adding a custom recognizer.

#### Transformer Model (`en_core_web_trf`) - High Accuracy English

Tests run with `en_core_web_trf` + multilingual fallback (28/29 passed):

| Language | Person Detection | Other PII | Latency |
|----------|-----------------|-----------|---------|
| 🇺🇸 English | ✅ John Smith | Email, IP, CC, SSN | 33-52ms |
| 🇨🇳 Chinese | ✅ 张三, 李明华, 欧阳明月 | Phone, ID Card | 35-50ms |
| 🇩🇪 German | ✅ Hans Müller, Berlin | - | 33-40ms |
| 🇫🇷 French | ✅ Pierre Dupont | - | 40-45ms |
| 🇪🇸 Spanish | ✅ Carlos García | - | 35-45ms |
| 🇯🇵 Japanese | ✅ 山田太郎 | - | 45-50ms |
| 🇷🇺 Russian | ❌ Not supported | - | N/A |
| 🇰🇷 Korean | ⚠️ Limited | - | N/A |

**Note:** `en_core_web_trf` is English-only (uses RoBERTa neural network). Russian detection requires the multilingual model.

#### Model Comparison Summary

| Feature | xx_ent_wiki_sm | en_core_web_trf |
|---------|---------------|-----------------|
| **Languages** | 100+ | English only |
| **Accuracy** | Good | Best (for English) |
| **Latency (CPU)** | ~3-5ms | ~35-65ms |
| **Latency (GPU)** | N/A | ~50-100ms |
| **Memory** | ~150MB | ~600MB + PyTorch |
| **Russian/Korean** | ✅ Russian / ⚠️ Korean | ❌ / ⚠️ |
| **Best for** | Multilingual apps | English-only + GPU |

### ⚠️ Transformer Model Warning

For **English-only** deployments requiring highest accuracy, you can use `en_core_web_trf`:

| Environment | Latency | Recommendation |
|-------------|---------|----------------|
| **With GPU** | ~50-100ms | Excellent for production |
| **Without GPU (CPU only)** | ~1-2 seconds | **Too slow for production** |

```bash
# Check if you have GPU
python -c "import torch; print('GPU:', torch.cuda.is_available())"

# For multilingual (default, recommended)
make pii-start

# For English-only with highest accuracy (requires GPU)
PRESIDIO_SPACY_MODEL=en_core_web_trf make pii-start
```

### Default Models

| Language | Model | Size | Notes |
|----------|-------|------|-------|
| Multilingual | `xx_ent_wiki_sm` | ~11MB | Default, 100+ languages |
| Chinese | Custom Regex | 0 MB | Built-in recognizers for names, phones, ID cards |

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

### XLM-RoBERTa Performance (Recommended for Chinese)

Benchmark results on standard CPU (no GPU required):

| Prompt Size | Latency (avg) | Throughput | PII Entities |
|-------------|---------------|------------|--------------|
| Short (~100 chars) | 44 ms | 3 KB/s | 4 |
| Medium (~1KB) | 83 ms | 13 KB/s | 2 |
| Long (~10KB) | 183 ms | 80 KB/s | 3 |
| Very Long (~100KB) | 232 ms | 441 KB/s | 4 |
| **Extreme (~500KB)** | **405 ms** | **904 KB/s** | 4 |

**Key Insights:**
- Latency scales **sub-linearly** with prompt size
- 500KB prompt processed in ~0.4 seconds
- Throughput improves with larger prompts (batch processing efficiency)
- **Memory usage: ~1.5GB** (includes transformers + model weights)

### spaCy Performance (Faster, Less Accurate)

| Text Length | Latency | Entities Detected |
|-------------|---------|-------------------|
| Short (~50 chars) | 3-5ms | 1-2 |
| Medium (~200 chars) | 5-10ms | 3-5 |
| Long (~500 chars) | 10-15ms | 5-10 |

### Memory Usage

| Engine | Base Memory | Model Size | Total |
|--------|-------------|------------|-------|
| spaCy (xx_ent_wiki_sm) | ~100MB | ~50MB | **~150MB** |
| spaCy (en_core_web_lg) | ~100MB | ~500MB | **~600MB** |
| XLM-RoBERTa | ~100MB | ~1.4GB | **~1.5GB** |

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

# Model Selection (affects accuracy and performance)
PRESIDIO_SPACY_MODEL=xx_ent_wiki_sm  # Options: xx_ent_wiki_sm (multilingual), en_core_web_lg, en_core_web_trf
                                      # Default: xx_ent_wiki_sm (100+ languages)
                                      # WARNING: en_core_web_trf is slow on CPU (~35-65ms per request)

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
  "redacted_input": "My name is [PERSON_a7f3e2], email [EMAIL_c9d5f3]",
  "detections": [...],
  "entities": [
    {"type": "PERSON", "mask": "[PERSON_a7f3e2]", "start": 11, "end": 21, "confidence": 0.85},
    {"type": "EMAIL_ADDRESS", "mask": "[EMAIL_c9d5f3]", "start": 29, "end": 45, "confidence": 1.0}
  ],
  "annotations": {
    "pii_count": 2,
    "pii_types": ["PERSON", "EMAIL_ADDRESS"],
    "processing_time_ms": 15
  }
}
```

### Unique Token Format

Each PII entity is replaced with a **unique token** in the format `[TYPE_hash]`:

| Original | Token | Purpose |
|----------|-------|---------|
| John Smith | `[PERSON_a7f3e2]` | First person detected |
| Jane Doe | `[PERSON_b8c4d1]` | Second person (different hash) |
| john@example.com | `[EMAIL_c9d5f3]` | Email address |
| 110101199001011234 | `[CNID_26b9cf]` | Chinese ID card |

**Why unique tokens?**
- Multiple PII of the same type can be distinguished
- LLM responses can reference specific tokens
- Gateway can restore the correct original value

**Example with multiple persons:**
```
Input:  "会议参与者：张三和李四"
Output: "会议参与者：[PERSON_2dc9a2]和[PERSON_f925ec]"

Token mapping:
  [PERSON_2dc9a2] → 张三
  [PERSON_f925ec] → 李四
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
# Using make commands (recommended)
make pii-model-lg

# Or manually
source venv/bin/activate
python -m spacy download en_core_web_lg
```

### Transformer model is slower than multilingual model
The transformer model (`en_core_web_trf`) takes ~35-65ms per request on CPU (vs ~3-5ms for `xx_ent_wiki_sm`). This is expected because it uses a RoBERTa neural network.

**Solution:** Use the default multilingual model for best latency:
```bash
# Use default multilingual model (fastest, 100+ languages)
make pii-stop && make pii-start

# Or explicitly set the model
PRESIDIO_SPACY_MODEL=xx_ent_wiki_sm make pii-start
```

### High memory usage
Use a smaller model:
```bash
make pii-model-sm  # Download 12MB model
PRESIDIO_SPACY_MODEL=en_core_web_sm make pii-start
```

### Slow first request
This is normal - models are loaded on first request. Subsequent requests are fast.

### Chinese names not detected
Ensure `PRESIDIO_ENABLE_CHINESE=true` (default).

### Check GPU availability
```bash
cd examples/firewall/presidio_sidecar
source venv/bin/activate
python -c "import torch; print('GPU available:', torch.cuda.is_available())"
```

## Testing

```bash
# Multilingual PII detection tests (Presidio sidecar only)
make pii-test

# End-to-end tests (Gateway + Presidio via OpenAI/Anthropic APIs)
./tests/firewall/test_e2e_pii_firewall.sh --setup
```

### E2E Test Results

End-to-end tests via Tokligence Gateway (all passed):

| API Endpoint | Tests | Status |
|--------------|-------|--------|
| Presidio Direct (`/v1/filter/input`) | Chinese name, Email, ID card | ✅ 3/3 |
| OpenAI Chat (`/v1/chat/completions`) | No PII, Email, Chinese name | ✅ 3/3 |
| Anthropic Messages (`/anthropic/v1/messages`) | No PII, Phone number | ✅ 2/2 |

**Note:** E2E tests require:
1. Gateway running with firewall enabled (`TOKLIGENCE_PROMPT_FIREWALL_CONFIG`)
2. Presidio sidecar running on port 7317
3. Valid API keys for OpenAI/Anthropic in `.env`

### Firewall Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `monitor` | Log PII detections, allow requests | Development, debugging |
| `redact` | Replace PII with tokens, restore in responses | **Production (recommended)** |
| `enforce` | Block requests containing critical PII | High-security environments |

### Redact Mode Details

In `redact` mode, the firewall:
1. Detects PII in user prompts (via Presidio + built-in regex)
2. Replaces PII with tokens (e.g., `john@example.com` → `[EMAIL_TOKEN_abc123]`)
3. Sends redacted prompt to LLM
4. Restores original PII in LLM responses (if tokens appear)

**Token Storage Configuration:**
```ini
[tokenizer]
store_type = memory    # memory | redis | redis_cluster
ttl = 1h               # Token mapping TTL (default: 1 hour)
```

**Storage Options:**
- `memory`: Fast, single-instance, lost on restart (default for dev)
- `redis`: Persistent, supports multiple gateway instances
- `redis_cluster`: High availability, distributed

## License

MIT
