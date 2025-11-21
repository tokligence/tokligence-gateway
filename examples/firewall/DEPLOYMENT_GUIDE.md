# Firewall Deployment Guide

本指南说明如何在实际环境中部署 Prompt Firewall。

## 部署架构

```
┌─────────────────────────────────────────┐
│  用户下载安装 Tokligence Gateway         │
│                                         │
│  1. Git clone / Download release        │
│  2. make build                          │
│  3. 配置 firewall (可选)                 │
│     ├─ 仅内置过滤器 (无需额外安装)        │
│     └─ 或启用 Presidio (需要 Python)     │
└─────────────────────────────────────────┘
```

## 部署选项

### 选项 1: 仅内置过滤器（推荐新手）

**优点**:
- ✅ 无需安装额外依赖
- ✅ 零配置开箱即用
- ✅ 延迟极低（5-10ms）
- ✅ 适合大部分场景

**缺点**:
- ❌ 准确率较低（~85%）
- ❌ 仅支持基础 PII 类型

**步骤**:

```bash
# 1. 克隆仓库
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# 2. 构建
make build

# 3. 复制配置（使用内置过滤器）
cp examples/firewall/configs/firewall.yaml config/

# 4. 启动
make gds

# 完成！内置过滤器自动运行
```

**配置文件** (`config/firewall.yaml`):
```yaml
enabled: true
mode: monitor  # 先监控，确认无误后改为 enforce

input_filters:
  - type: pii_regex        # 内置过滤器，无需外部依赖
    name: input_pii
    priority: 10
    enabled: true
    config:
      redact_enabled: false
      enabled_types:
        - EMAIL
        - PHONE
        - SSN
        - CREDIT_CARD
```

### 选项 2: 内置 + Presidio（推荐生产环境）

**优点**:
- ✅ 准确率高（~95%）
- ✅ 支持 15+ PII 类型
- ✅ 多层防护

**缺点**:
- ❌ 需要 Python 3.8+
- ❌ 额外内存占用（~1GB）
- ❌ 延迟增加（50-200ms）

**步骤**:

```bash
# 1-2. 同上（克隆、构建）

# 3. 设置 Presidio sidecar（独立 venv 环境）
cd examples/firewall/presidio_sidecar
./setup.sh
# 这会创建 venv/ 目录并安装依赖

# 4. 启动 Presidio
./start.sh
# 服务运行在 http://localhost:7317

# 5. 验证 Presidio 正常运行
curl http://localhost:7317/health
# 应返回: {"status": "healthy", ...}

# 6. 配置 gateway 使用 Presidio
cd ../../..  # 回到项目根目录
cp examples/firewall/configs/firewall-enforce.yaml config/firewall.yaml

# 7. 启动 gateway
make gds

# 完成！
```

## Presidio Sidecar 详解

### 安装位置

```
tokligence-gateway/
└── examples/
    └── firewall/
        └── presidio_sidecar/
            ├── venv/              # Python 虚拟环境（setup.sh 创建）
            ├── main.py            # FastAPI 服务
            ├── requirements.txt   # Python 依赖
            ├── setup.sh           # 安装脚本
            ├── start.sh           # 启动脚本
            ├── stop.sh            # 停止脚本
            └── presidio.log       # 运行日志
```

### 环境隔离

**Presidio 使用独立的 venv 环境，不会污染全局 Python**:

```bash
# 自动管理（推荐）
./setup.sh    # 创建 venv + 安装依赖
./start.sh    # 自动激活 venv 并启动
./stop.sh     # 停止服务

# 手动管理
source venv/bin/activate     # 激活环境
python main.py               # 启动服务
deactivate                   # 退出环境
```

### 启动方式

#### 方式 1: 使用脚本（推荐）

```bash
cd examples/firewall/presidio_sidecar

# 启动
./start.sh
# ✓ Presidio sidecar started successfully (PID: 12345)
#   Logs: /path/to/presidio.log
#   Health check: curl http://localhost:7317/health

# 停止
./stop.sh
# ✓ Presidio sidecar stopped
```

#### 方式 2: 手动启动

```bash
cd examples/firewall/presidio_sidecar
source venv/bin/activate
python main.py

# 或后台运行
nohup python main.py > presidio.log 2>&1 &
```

#### 方式 3: Systemd 服务（生产环境）

```bash
# 创建 systemd 服务文件
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

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable presidio-firewall
sudo systemctl start presidio-firewall

# 查看状态
sudo systemctl status presidio-firewall
```

#### 方式 4: Docker（推荐生产）

```bash
cd examples/firewall/presidio_sidecar

# 构建镜像
docker build -t presidio-firewall .

# 运行容器
docker run -d \
  --name presidio-firewall \
  -p 7317:7317 \
  --restart unless-stopped \
  presidio-firewall

# 查看日志
docker logs -f presidio-firewall
```

### Docker Compose 完整部署

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

启动:
```bash
docker-compose up -d
```

## 用户安装流程

### 场景 1: 开发者本地试用

```bash
# 下载
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# 构建
make build

# 默认配置（内置过滤器）
make gds

# 测试
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

### 场景 2: 需要高精度 PII 检测

```bash
# 1-2. 同上

# 3. 安装 Presidio（一次性）
cd examples/firewall/presidio_sidecar
./setup.sh        # 创建 venv，安装依赖（5-10 分钟）

# 4. 启动 Presidio
./start.sh        # 后台运行

# 5. 配置使用 Presidio
cd ../../..
cp examples/firewall/configs/firewall-enforce.yaml config/firewall.yaml

# 6. 启动 gateway
make gds
```

### 场景 3: 生产环境部署

```bash
# 1. 服务器上克隆代码
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway

# 2. 构建
make build

# 3. 设置 Presidio systemd 服务
cd examples/firewall/presidio_sidecar
./setup.sh
# 然后设置 systemd（见上文）

# 4. 配置 firewall
cd ../../..
cp examples/firewall/configs/firewall-enforce.yaml config/firewall.yaml
# 编辑 config/firewall.yaml 调整策略

# 5. 设置 gateway systemd 服务
# (参考 gateway 的部署文档)

# 6. 启动所有服务
sudo systemctl start presidio-firewall
sudo systemctl start tokligence-gateway
```

## 配置策略建议

### 第一周：监控模式

```yaml
enabled: true
mode: monitor       # 只记录，不阻断
input_filters:
  - type: pii_regex
    enabled: true
    config:
      redact_enabled: false  # 不修改内容
```

**目的**: 了解流量中的 PII 模式，收集数据

### 第二周：调优

```bash
# 分析日志
grep firewall logs/gatewayd.log | grep pii_count

# 调整配置
# 如果误报多，减少 enabled_types
# 如果漏报多，启用 Presidio
```

### 第三周：强制模式

```yaml
enabled: true
mode: enforce       # 开始阻断
input_filters:
  - type: pii_regex
    enabled: true
    config:
      redact_enabled: true   # 自动脱敏

output_filters:
  - type: pii_regex
    enabled: true
    config:
      redact_enabled: true   # 输出也脱敏
```

## 常见问题

### Q: Presidio 是必须的吗？

**A**: 不是。内置正则过滤器对大部分场景已经够用。Presidio 适合：
- 需要高准确率（金融、医疗等）
- 需要检测人名、地址等复杂 PII
- 有 Python 环境和足够资源

### Q: Presidio 占用多少资源？

**A**:
- 内存: ~500MB-1GB（取决于模型大小）
- CPU: 0.5-1 核心
- 磁盘: ~500MB（模型 + 依赖）

### Q: 可以不用 venv 吗？

**A**: 不推荐。venv 可以：
- 隔离依赖，避免版本冲突
- 方便卸载（直接删除 venv/ 目录）
- 不影响系统 Python 环境

### Q: 如何验证 firewall 正常工作？

**A**:
```bash
# 1. 检查 gateway 日志
grep "firewall configured" logs/gatewayd.log
# 应该看到: firewall configured: mode=monitor filters=2

# 2. 发送包含 PII 的请求
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"My email is test@example.com"}]}'

# 3. 检查检测日志
grep "firewall.monitor" logs/gatewayd.log
# 应该看到: [firewall.monitor] location=input pii_count=1 types=[EMAIL]
```

### Q: Presidio 启动失败怎么办？

**A**:
```bash
# 检查日志
tail -f examples/firewall/presidio_sidecar/presidio.log

# 常见问题：
# 1. 缺少 spaCy 模型
python -m spacy download en_core_web_lg

# 2. 端口被占用
lsof -i :7317
# 杀掉占用进程或换端口

# 3. 内存不足
# 使用小模型: python -m spacy download en_core_web_sm
# 修改 main.py 使用 en_core_web_sm
```

### Q: 生产环境推荐配置？

**A**:
```yaml
# 推荐配置
enabled: true
mode: enforce

input_filters:
  # 快速预过滤
  - type: pii_regex
    priority: 5
    enabled: true
    config:
      redact_enabled: false

  # 深度检测
  - type: http
    priority: 10
    enabled: true
    config:
      endpoint: http://localhost:7317/v1/filter/input
      timeout_ms: 300        # 快速超时
      on_error: allow        # 服务故障时继续（优雅降级）

output_filters:
  # 输出必须脱敏
  - type: pii_regex
    enabled: true
    config:
      redact_enabled: true

policies:
  redact_pii: true
  max_pii_entities: 3
```

## 性能调优

### 高吞吐量场景

```yaml
# 1. 仅用内置过滤器
input_filters:
  - type: pii_regex
    enabled: true

# 2. 或增加 Presidio 超时
config:
  timeout_ms: 200           # 快速超时
  on_error: bypass          # 超时时跳过
```

### 高准确率场景

```yaml
# 1. 启用 Presidio
- type: http
  enabled: true
  config:
    timeout_ms: 1000        # 更长超时
    on_error: block         # 服务故障时阻断（fail-closed）

# 2. 部署多个 Presidio 实例 + 负载均衡
endpoint: http://presidio-lb:7317/v1/filter/input
```

## 卸载

```bash
# 停止服务
cd examples/firewall/presidio_sidecar
./stop.sh

# 删除 venv（可选）
rm -rf venv/

# 禁用 firewall
# 编辑 config/firewall.yaml
enabled: false
```

## 总结

**内置过滤器**:
- ✅ 零配置，开箱即用
- ✅ 性能最佳
- ✅ 适合大部分场景

**Presidio**:
- ✅ 准确率高
- ✅ 支持更多 PII 类型
- ❌ 需要额外安装
- ❌ 占用更多资源

**建议**:
1. 先用内置过滤器试用
2. 监控模式运行 1-2 周
3. 根据需求决定是否启用 Presidio
4. 生产环境使用混合模式 + 优雅降级
