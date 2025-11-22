# Firewall Redact Mode - PII Tokenization

## 概述

**Redact模式**是Prompt Firewall的第四种运行模式，它会：
1. 检测PII（个人身份信息）
2. 替换为假的但格式一致的token
3. 将映射关系保存（内存或Redis）
4. LLM收到的是去除PII的内容
5. LLM响应返回时，自动替换回真实PII
6. 用户无感知，可直接复制粘贴

## 四种模式对比

| 模式 | 行为 | 用途 | 性能影响 |
|------|------|------|---------|
| `monitor` | 检测并记录，不修改内容 | 观察PII模式，收集数据 | ~5-10ms |
| `enforce` | 检测到PII后阻断请求 | 严格禁止PII | ~5-10ms |
| **`redact`** | **检测→替换token→映射回原值** | **允许PII但保护隐私** | **~10-20ms** |
| `disabled` | 关闭firewall | 不需要保护 | 0ms |

## 工作原理

### 1. Input阶段（用户→LLM）

**用户输入**:
```
我的邮箱是 john.doe@example.com，电话是 555-123-4567
```

**Firewall处理**:
- 检测到EMAIL: `john.doe@example.com`
- 检测到PHONE: `555-123-4567`
- 生成tokens:
  - `john.doe@example.com` → `user_a7f3e2@redacted.local`
  - `555-123-4567` → `+1-555-a7f-3e2d`
- 保存mapping到store（内存/Redis）

**LLM收到**:
```
我的邮箱是 user_a7f3e2@redacted.local，电话是 +1-555-a7f-3e2d
```

### 2. Output阶段（LLM→用户）

**LLM返回**:
```
好的，我会发送邮件到 user_a7f3e2@redacted.local 并致电 +1-555-a7f-3e2d 确认
```

**Firewall处理**:
- 查找所有tokens
- 替换回原始值

**用户看到**:
```
好的，我会发送邮件到 john.doe@example.com 并致电 555-123-4567 确认
```

## Token生成策略

### Email
- **输入**: `john.doe@example.com`
- **Token**: `user_a7f3e2@redacted.local`
- **规则**: `user_{hash7}@redacted.local`

### Phone
- **输入**: `555-123-4567`
- **Token**: `+1-555-a7f-3e2d`
- **规则**: `+1-555-{hash3}-{hash4}`

### SSN
- **输入**: `123-45-6789`
- **Token**: `XXX-XX-a7f3`
- **规则**: `XXX-XX-{hash4}`

### Credit Card
- **输入**: `4532-1234-5678-9012`
- **Token**: `XXXX-XXXX-XXXX-a7f3`
- **规则**: `XXXX-XXXX-XXXX-{hash4}`

### IP Address
- **输入**: `192.168.1.100`
- **Token**: `10.0.a7.f3`
- **规则**: `10.0.{hash2}.{hash2}`

### API Key
- **输入**: `sk-proj-abc123def456...`
- **Token**: `sk-redacted-a7f3e2d4c1b9`
- **规则**: `sk-redacted-{hash12}`

## 配置示例

### 基础配置（内存存储）

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact  # 关键：使用redact模式

# PII patterns configuration file
pii_patterns_file = config/pii_patterns.ini

# Enabled regions
pii_regions = global,us,cn

# Log settings
log_decisions = true
log_pii_values = false

# Maximum PII entities in a single request
max_pii_entities = 50

# Tokenizer配置
[tokenizer]
store_type = memory  # memory | redis | redis_cluster
ttl = 1h             # Token mapping有效期

# Input filters
[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

# Output filters
[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

### Redis存储（企业级/分布式）

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact

pii_patterns_file = config/pii_patterns.ini
pii_regions = global,us
log_decisions = true

[tokenizer]
store_type = redis
ttl = 2h

# Redis configuration
redis_addr = localhost:6379
redis_password =
redis_db = 0
redis_key_prefix = firewall:tokens

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

### Redis Cluster（高可用）

```ini
# config/firewall.ini
[prompt_firewall]
enabled = true
mode = redact

pii_patterns_file = config/pii_patterns.ini
pii_regions = global,us,cn

[tokenizer]
store_type = redis_cluster
ttl = 24h

# Redis Cluster configuration
redis_cluster_addrs = redis-node1:7000,redis-node2:7001,redis-node3:7002
redis_cluster_key_prefix = firewall:pii

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

## 使用场景

### 场景1：客服对话

**问题**: 客服需要在LLM帮助下处理客户信息，但不能将真实PII发送给LLM

**解决方案**:
```ini
[prompt_firewall]
mode = redact
pii_regions = global,us

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**效果**:
- 客服输入真实信息
- LLM看到假token
- 客服看到真实信息（自动还原）

### 场景2：医疗咨询

**问题**: 医生需要LLM帮助分析病例，但患者信息必须保密

**解决方案**:
```ini
[prompt_firewall]
mode = redact
pii_regions = global,us
max_pii_entities = 100

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
# 使用Presidio检测更多类型（可选）
# filter_presidio_enabled = true
# filter_presidio_priority = 20
# filter_presidio_endpoint = http://localhost:7317/v1/filter/input

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
```

**效果**:
- 检测姓名、地址、病历号等
- LLM分析去PII的病例
- 医生看到完整信息

### 场景3：法律文档分析

**问题**: 律师需要LLM分析合同，但客户信息不能泄露

**解决方案**:
```ini
[prompt_firewall]
mode = redact
pii_regions = global,us,eu

[tokenizer]
store_type = redis  # 使用Redis持久化
ttl = 7d            # 保留7天用于审计
redis_addr = localhost:6379
redis_key_prefix = firewall:legal

[firewall_input_filters]
filter_pii_regex_enabled = true

[firewall_output_filters]
filter_pii_regex_enabled = true
```

## Token存储架构

### 内存存储（单实例）

```
┌──────────────────────────────────────┐
│  Tokligence Gateway                  │
│  ┌────────────────────────────────┐ │
│  │  MemoryTokenStore              │ │
│  │  ├─ session-123                │ │
│  │  │  ├─ EMAIL                   │ │
│  │  │  │  └─ john@...→user_a7f... │ │
│  │  │  └─ PHONE                   │ │
│  │  │     └─ 555-...→+1-555-...   │ │
│  └────────────────────────────────┘ │
└──────────────────────────────────────┘
```

**优点**: 快速，无外部依赖
**缺点**: 重启丢失，不支持多实例

### Redis存储（分布式）

```
┌──────────────┐     ┌──────────────┐
│  Gateway-1   │────▶│              │
└──────────────┘     │              │
                     │    Redis     │
┌──────────────┐     │   Cluster    │
│  Gateway-2   │────▶│              │
└──────────────┘     │              │
                     │              │
┌──────────────┐     │              │
│  Gateway-3   │────▶│              │
└──────────────┘     └──────────────┘
```

**优点**: 持久化，支持多实例，高可用
**缺点**: 需要Redis基础设施

## Session管理

### Session ID生成

每个请求都有唯一的session ID，用于隔离不同对话的PII mapping：

```go
// 使用request ID作为session ID
sessionID := req.RequestID

// 或使用用户ID + timestamp
sessionID := fmt.Sprintf("%s:%d", userID, time.Now().Unix())
```

### TTL（Time To Live）

Token映射会在TTL后自动过期：

```ini
[tokenizer]
ttl = 1h  # 1小时后自动清理
```

### 手动清理

```go
// 在代码中清理特定session
tokenizer.CleanupSession(ctx, sessionID)

// 清理所有过期session
tokenizer.CleanupExpired(ctx)
```

## 性能考虑

### 延迟影响

| 操作 | 内存存储 | Redis本地 | Redis远程 |
|------|---------|----------|----------|
| 检测PII | ~5ms | ~5ms | ~5ms |
| 生成Token | ~1ms | ~1ms | ~1ms |
| 存储Mapping | <0.1ms | ~1ms | ~5ms |
| 查询Mapping | <0.1ms | ~1ms | ~5ms |
| **总延迟** | **~10ms** | **~15ms** | **~20ms** |

### 吞吐量

- **内存存储**: 10K+ req/s
- **Redis本地**: 5K+ req/s
- **Redis集群**: 3K+ req/s

### 内存占用

每个token mapping约100-200字节：
- 1K active sessions × 5 tokens/session = ~1MB
- 100K sessions = ~100MB

## 安全考虑

### Token碰撞

使用 `md5(originalValue + timestamp + random)` 确保token唯一性：
- 碰撞概率: < 1/10^12
- 即使相同PII，不同session也有不同token

### Mapping泄露防护

1. **Redis密码保护**
```yaml
tokenizer:
  redis:
    password: "strong-password"
```

2. **网络隔离**
```yaml
tokenizer:
  redis:
    addr: internal-redis:6379  # 内网地址
```

3. **TTL自动清理**
```yaml
tokenizer:
  ttl: 30m  # 30分钟后自动清理
```

## 监控和调试

### 查看Mappings

```bash
# 在debug模式下，日志会显示mappings
TOKLIGENCE_LOG_LEVEL=debug make gfr

# 日志输出示例：
# [firewall.redact] session=req-123 pii_type=EMAIL original=john@example.com token=user_a7f3e2@redacted.local
# [firewall.redact] session=req-123 detokenize: 2 tokens restored
```

### 健康检查

```bash
# 查看token store状态
curl http://localhost:8079/admin/firewall/stats

# 响应示例：
{
  "mode": "redact",
  "active_sessions": 125,
  "total_tokens": 673,
  "store_type": "redis",
  "avg_latency_ms": 15
}
```

## 故障排查

### Token未被还原

**症状**: LLM响应中仍然显示token（如`user_a7f3e2@redacted.local`）

**原因**:
1. Session ID不一致
2. Token已过期被清理
3. Output filter未启用

**解决**:
```ini
# 确保output filter启用
[firewall_output_filters]
filter_pii_regex_enabled = true  # 必须为true
```

### Redis连接失败

**症状**: `RedisTokenStore not fully implemented`

**原因**: Redis store是骨架实现，需要添加go-redis依赖

**解决**:
```bash
# 1. 添加依赖
go get github.com/redis/go-redis/v9

# 2. 完善token_store_redis.go实现
# 3. 重新编译
make build
```

### 性能下降

**症状**: 请求延迟显著增加

**原因**: Redis网络延迟

**解决**:
```ini
# 1. 使用本地Redis
[tokenizer]
redis_addr = localhost:6379

# 2. 或切换到内存存储
[tokenizer]
store_type = memory

# 3. 调整TTL减少存储
[tokenizer]
ttl = 15m  # 缩短到15分钟
```

## 最佳实践

### 1. 选择合适的存储

- **开发环境**: 内存存储
- **单实例生产**: 内存存储 + 定期备份
- **多实例生产**: Redis
- **高可用要求**: Redis Cluster

### 2. 设置合理的TTL

- **短会话**: 30分钟
- **正常会话**: 1-2小时
- **需要审计**: 7-30天（Redis持久化）

### 3. 监控存储增长

```bash
# 定期检查Redis内存使用
redis-cli INFO memory

# 设置最大内存限制
redis-cli CONFIG SET maxmemory 1gb
redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

### 4. 日志脱敏

确保日志中不记录原始PII：
```ini
# 只记录检测事件，不记录原始值
[prompt_firewall]
log_decisions = true
log_pii_values = false  # 不记录真实PII
```

## 下一步

- [ ] 完善Redis store实现（添加go-redis依赖）
- [ ] 更新PII regex filter支持redact模式
- [ ] 添加redact模式集成测试
- [ ] 性能基准测试
- [ ] 监控dashboard

## 参考

- [Firewall主文档](PROMPT_FIREWALL.md)
- [部署指南](../examples/firewall/DEPLOYMENT_GUIDE.md)
- [性能调优](../examples/firewall/PERFORMANCE_TUNING.md)
