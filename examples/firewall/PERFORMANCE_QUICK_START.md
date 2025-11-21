# Performance Quick Start

## 问题

Go Gateway 很快（10K+ req/s），但 Python Presidio 可能成为瓶颈（~150 req/s）。

## 快速解决方案

### 个人用户（<100 req/s）

只用内置过滤器，不需要 Presidio：

```yaml
# config/firewall.yaml
enabled: true
mode: monitor

input_filters:
  - type: pii_regex
    enabled: true
```

**性能**: 10K+ req/s ✅

---

### 小团队（100-1000 req/s）

启用多进程 Presidio：

```bash
# 1. 安装 Presidio
cd examples/firewall/presidio_sidecar
./setup.sh

# 2. 设置 Workers（根据 CPU 核心数）
export PRESIDIO_WORKERS=8  # 8 核 CPU

# 3. 启动
./start.sh

# 4. 配置 Gateway
cp examples/firewall/configs/firewall-enforce.yaml config/firewall.yaml

# 5. 启动 Gateway
make gds
```

**性能**: ~1200 req/s ✅

---

### 企业级（1000+ req/s）

多实例 + 负载均衡：

```bash
# Docker Compose 部署
cd examples/firewall
docker-compose -f docker-compose.high-performance.yml up -d
```

**性能**: ~2000 req/s ✅

---

### 最佳性能（10K+ req/s）

混合策略（内置 + Presidio + 降级）：

```yaml
# config/firewall.yaml
enabled: true
mode: enforce

input_filters:
  # 快速路径（处理 80% 的情况）
  - type: pii_regex
    priority: 5
    enabled: true
    config:
      redact_enabled: false

  # 深度分析（仅处理复杂情况）
  - type: http
    priority: 10
    enabled: true
    config:
      endpoint: http://localhost:7317/v1/filter/input
      timeout_ms: 200  # 快速超时
      on_error: allow  # 超时时降级
```

**性能**: 10K+ req/s ✅

---

## 性能对比表

| 配置 | 吞吐量 | 适用场景 |
|------|--------|---------|
| 仅内置过滤器 | 10K+ req/s | 个人用户 |
| Presidio (1 worker) | ~150 req/s | 测试环境 |
| Presidio (4 workers) | ~600 req/s | 小团队 |
| Presidio (8 workers) | ~1200 req/s | 中型团队 |
| 4 实例 + LB | ~2000 req/s | 企业级 |
| 混合策略 | 10K+ req/s | 高性能需求 |

---

## 快速测试

```bash
# 1. 测试内置过滤器
./examples/firewall/test_firewall.sh

# 2. 压力测试
go install github.com/rakyll/hey@latest

hey -n 10000 -c 100 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"input":"test@example.com"}' \
  http://localhost:7317/v1/filter/input
```

---

## 配置 Workers 数量

```bash
# 查看 CPU 核心数
nproc  # Linux
sysctl -n hw.ncpu  # macOS

# 设置 Workers = CPU 核心数 × 2
export PRESIDIO_WORKERS=16  # 8 核 CPU × 2
./start.sh
```

---

**完整文档**: `examples/firewall/PERFORMANCE_TUNING.md`
