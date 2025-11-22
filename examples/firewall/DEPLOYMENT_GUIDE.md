# Prompt Firewall Deployment Guide

This guide explains how to deploy the Prompt Firewall in real-world environments.

## Deployment Architecture

```
┌──────────────────────────────────────────────┐
│  Tokligence Gateway Installation             │
│                                              │
│  1. Git clone / Download release             │
│  2. make build                               │
│  3. Configure firewall (optional)            │
│     ├─ Built-in filters only (no setup)      │
│     └─ or Enable Presidio (requires Python)  │
└──────────────────────────────────────────────┘
```

## Deployment Options

### Option 1: Built-in Filters Only (Recommended for Beginners)

**Advantages**:
- ✅ No additional dependencies required
- ✅ Zero configuration, works out of the box
- ✅ Ultra-low latency (~5-10ms)
- ✅ Suitable for most use cases

**Limitations**:
- ❌ Lower accuracy (~85%)
- ❌ Supports basic PII types only

**Steps**:

```bash
# 1. Clone repository
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# 2. Build
make build

# 3. Copy firewall configuration (uses built-in filters)
cp examples/firewall/configs/firewall.ini config/

# 4. Start gateway
make gds

# Done! Built-in filters are now active
```

**Configuration File** (`config/firewall.ini`):
```ini
[prompt_firewall]
enabled = true
mode = redact  # redact (recommended) | monitor | enforce | disabled

# PII patterns configuration
pii_patterns_file = config/pii_patterns.ini
pii_regions = global,us,cn

# Logging
log_decisions = true
log_pii_values = false

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

### Option 2: Built-in + Presidio (Recommended for Production)

**Advantages**:
- ✅ High accuracy (~95%)
- ✅ Supports 15+ PII types
- ✅ Multi-layer protection

**Limitations**:
- ❌ Requires Python 3.8+
- ❌ Additional memory usage (~1GB)
- ❌ Increased latency (~50-200ms)

**Steps**:

```bash
# 1-2. Same as above (clone, build)

# 3. Setup Presidio sidecar (isolated venv environment)
cd examples/firewall/presidio_sidecar
./setup.sh
# This creates a venv/ directory and installs dependencies

# 4. Start Presidio
./start.sh
# Service runs on http://localhost:7317

# 5. Verify Presidio is running
curl http://localhost:7317/health
# Should return: {"status": "healthy", ...}

# 6. Configure gateway to use Presidio
cd ../../..  # Back to project root
cp examples/firewall/configs/firewall-enforce.ini config/firewall.ini

# 7. Start gateway
make gds

# Done!
```

## Presidio Sidecar Details

### Installation Location

```
tokligence-gateway/
└── examples/
    └── firewall/
        └── presidio_sidecar/
            ├── venv/              # Python virtual environment (created by setup.sh)
            ├── main.py            # FastAPI service
            ├── requirements.txt   # Python dependencies
            ├── setup.sh           # Installation script
            ├── start.sh           # Start script
            ├── stop.sh            # Stop script
            └── presidio.log       # Runtime log
```

### Environment Isolation

**Presidio uses an isolated venv environment and does not pollute global Python**:

```bash
# Automatic management (recommended)
./setup.sh    # Create venv + install dependencies
./start.sh    # Auto-activate venv and start
./stop.sh     # Stop service

# Manual management
source venv/bin/activate     # Activate environment
python main.py               # Start service
deactivate                   # Exit environment
```

### Startup Methods

#### Method 1: Using Scripts (Recommended)

```bash
cd examples/firewall/presidio_sidecar

# Start
./start.sh
# ✓ Presidio sidecar started successfully (PID: 12345)
#   Logs: /path/to/presidio.log
#   Health check: curl http://localhost:7317/health

# Stop
./stop.sh
# ✓ Presidio sidecar stopped
```

#### Method 2: Manual Start

```bash
cd examples/firewall/presidio_sidecar
source venv/bin/activate
python main.py

# Or run in background
nohup python main.py > presidio.log 2>&1 &
```

#### Method 3: Systemd Service (Production Environment)

```bash
# Create systemd service file
sudo tee /etc/systemd/system/presidio-firewall.service > /dev/null <<EOF
[Unit]
Description=Presidio Prompt Firewall Sidecar
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$(pwd)/examples/firewall/presidio_sidecar
ExecStart=$(pwd)/examples/firewall/presidio_sidecar/venv/bin/python main.py
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable presidio-firewall
sudo systemctl start presidio-firewall

# Check status
sudo systemctl status presidio-firewall
```

#### Method 4: Docker (Recommended for Production)

```bash
cd examples/firewall/presidio_sidecar

# Build image
docker build -t presidio-firewall .

# Run container
docker run -d \
  --name presidio-firewall \
  -p 7317:7317 \
  --restart unless-stopped \
  presidio-firewall

# View logs
docker logs -f presidio-firewall
```

### Docker Compose Full Deployment

```yaml
# docker-compose.yml
version: '3.8'

services:
  gateway:
    build: .
    ports:
      - "8081:8081"
    volumes:
      - ./config:/app/config
      - ./logs:/app/logs
    environment:
      - TOKLIGENCE_LOG_LEVEL=info
      - TOKLIGENCE_PROMPT_FIREWALL_ENABLED=true
      - TOKLIGENCE_PROMPT_FIREWALL_MODE=redact
    depends_on:
      presidio:
        condition: service_healthy

  presidio:
    build: ./examples/firewall/presidio_sidecar
    ports:
      - "7317:7317"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7317/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
```

Start:
```bash
docker-compose up -d
```

## User Installation Workflow

### Scenario 1: Local Development/Testing

```bash
# Download
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# Build
make build

# Use default config (built-in filters)
make gds

# Test
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

### Scenario 2: High-Accuracy PII Detection Required

```bash
# 1-2. Same as above

# 3. Install Presidio (one-time setup)
cd examples/firewall/presidio_sidecar
./setup.sh        # Create venv, install dependencies (5-10 minutes)

# 4. Start Presidio
./start.sh        # Run in background

# 5. Configure to use Presidio
cd ../../..
cp examples/firewall/configs/firewall-enforce.ini config/firewall.ini

# 6. Start gateway
make gds
```

### Scenario 3: Production Environment

```bash
# 1. Clone on server
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# 2. Build
make build

# 3. Setup Presidio systemd service
cd examples/firewall/presidio_sidecar
./setup.sh
# Then configure systemd (see above)

# 4. Configure firewall
cd ../../..
cp examples/firewall/configs/firewall-enforce.ini config/firewall.ini
# Edit config/firewall.ini to adjust policies

# 5. Setup gateway systemd service
# (Refer to gateway deployment documentation)

# 6. Start all services
sudo systemctl start presidio-firewall
sudo systemctl start tokligence-gateway
```

## Configuration Strategy Recommendations

### Week 1: Monitor Mode

```ini
[prompt_firewall]
enabled = true
mode = monitor       # Log only, don't block

[firewall_input_filters]
filter_pii_regex_enabled = true

[firewall_output_filters]
filter_pii_regex_enabled = true
```

**Goal**: Understand PII patterns in your traffic, collect data

### Week 2: Tuning

```bash
# Analyze logs
grep firewall logs/gatewayd.log | grep pii_count

# Adjust configuration
# If too many false positives: reduce enabled types
# If too many false negatives: enable Presidio
```

### Week 3: Enforce Mode

```ini
[prompt_firewall]
enabled = true
mode = redact        # Start protecting

[firewall_input_filters]
filter_pii_regex_enabled = true

[firewall_output_filters]
filter_pii_regex_enabled = true
```

## Environment Variables

Override configuration with environment variables:

```bash
# Enable/disable firewall
export TOKLIGENCE_PROMPT_FIREWALL_ENABLED=true

# Set mode
export TOKLIGENCE_PROMPT_FIREWALL_MODE=redact  # redact|monitor|enforce|disabled

# Use different config file
export TOKLIGENCE_PROMPT_FIREWALL_CONFIG=/path/to/custom/firewall.ini

# Then start
make gds
```

## Common Questions

### Q: Is Presidio required?

**A**: No. Built-in regex filters are sufficient for most scenarios. Use Presidio when:
- High accuracy is required (finance, healthcare)
- Need to detect complex PII (names, addresses)
- Have Python environment and sufficient resources

### Q: How much resources does Presidio use?

**A**:
- Memory: ~500MB-1GB (depends on model size)
- CPU: 0.5-1 core
- Disk: ~500MB (models + dependencies)

### Q: Can I skip venv?

**A**: Not recommended. Using venv:
- Isolates dependencies, avoids version conflicts
- Easy to uninstall (just delete venv/ directory)
- Doesn't affect system Python environment

### Q: How to verify firewall is working?

**A**:
```bash
# 1. Check gateway logs
grep "firewall configured" logs/gatewayd.log
# Should see: firewall configured: mode=redact filters=2

# 2. Send request with PII
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"My email is test@example.com"}]}'

# 3. Check detection logs
grep "firewall" logs/gatewayd.log
# Should see: firewall.input.redacted or firewall.monitor
```

### Q: What if Presidio fails to start?

**A**:
```bash
# Check logs
tail -f examples/firewall/presidio_sidecar/presidio.log

# Common issues:
# 1. Missing spaCy model
python -m spacy download en_core_web_lg

# 2. Port already in use
lsof -i :7317
# Kill the process or change port

# 3. Out of memory
# Use smaller model: python -m spacy download en_core_web_sm
# Modify main.py to use en_core_web_sm
```

### Q: Recommended production configuration?

**A**:
```ini
# Recommended config
[prompt_firewall]
enabled = true
mode = redact

[firewall_input_filters]
# Fast pre-filter
filter_pii_regex_enabled = true
filter_pii_regex_priority = 5

# Deep analysis (optional)
# filter_presidio_enabled = true
# filter_presidio_priority = 10
# filter_presidio_endpoint = http://localhost:7317/v1/filter/input
# filter_presidio_timeout_ms = 300        # Fast timeout
# filter_presidio_on_error = allow        # Graceful degradation

[firewall_output_filters]
# Output must be protected
filter_pii_regex_enabled = true
```

## Performance Tuning

### High Throughput Scenarios

```ini
# 1. Use built-in filters only
[firewall_input_filters]
filter_pii_regex_enabled = true

# 2. Or increase Presidio timeout
# filter_presidio_timeout_ms = 200      # Fast timeout
# filter_presidio_on_error = bypass     # Skip on timeout
```

### High Accuracy Scenarios

```ini
# 1. Enable Presidio
[firewall_input_filters]
filter_presidio_enabled = true
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 1000       # Longer timeout
filter_presidio_on_error = block        # Fail-closed

# 2. Deploy multiple Presidio instances + load balancer
# filter_presidio_endpoint = http://presidio-lb:7317/v1/filter/input
```

## Uninstall

```bash
# Stop services
cd examples/firewall/presidio_sidecar
./stop.sh

# Delete venv (optional)
rm -rf venv/

# Disable firewall
# Edit config/firewall.ini:
# enabled = false
```

## Summary

**Built-in Filters**:
- ✅ Zero configuration, works out of box
- ✅ Best performance
- ✅ Suitable for most scenarios

**Presidio**:
- ✅ High accuracy
- ✅ Supports more PII types
- ❌ Requires additional installation
- ❌ Uses more resources

**Recommendations**:
1. Start with built-in filters
2. Run in monitor mode for 1-2 weeks
3. Decide whether to enable Presidio based on requirements
4. Use hybrid mode + graceful degradation for production
