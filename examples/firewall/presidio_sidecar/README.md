# Presidio Prompt Firewall Sidecar

这是一个基于 Python 的 sidecar 服务，集成了 Microsoft Presidio 来为 Tokligence Gateway 提供 PII 检测和匿名化功能。

## 概述

**Presidio 是可选组件**，不是必须的：
- ✅ **内置过滤器已经够用** - 正则检测对大部分场景已足够
- ✅ **独立安装** - 使用虚拟环境，不污染全局 Python
- ✅ **按需启用** - 可以只在需要高精度时使用

## 功能特性

- **PII 检测**: 检测 15+ 种 PII 类型（邮箱、电话、SSN、信用卡、IP 等）
- **匿名化**: 自动脱敏或掩码
- **REST API**: 兼容 Tokligence Gateway 的 HTTP filter
- **低延迟**: 大部分请求 <200ms
- **可配置**: 容易自定义实体类型和阈值

## 快速开始

### 方式 1: 使用脚本（推荐）

```bash
# 一键安装（创建 venv + 安装依赖）
./setup.sh

# 启动服务
./start.sh

# 停止服务
./stop.sh
```

### 方式 2: 手动安装

```bash
# 创建虚拟环境（隔离依赖）
python3 -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# 安装依赖
pip install -r requirements.txt

# 下载 spaCy 语言模型（Presidio 需要）
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

```yaml
firewall:
  enabled: true
  mode: enforce
  input_filters:
    - type: http
      name: presidio_input
      priority: 5
      enabled: true
      config:
        endpoint: http://localhost:7317/v1/filter/input
        timeout_ms: 500
        on_error: allow
  output_filters:
    - type: http
      name: presidio_output
      priority: 5
      enabled: true
      config:
        endpoint: http://localhost:7317/v1/filter/output
        timeout_ms: 500
        on_error: bypass
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
