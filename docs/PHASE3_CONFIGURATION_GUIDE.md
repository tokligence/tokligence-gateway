# Phase 3 时间动态规则 - 配置示例

## 配置概述

Phase 3 支持 3 种规则类型：
1. **Weight Adjustment** (权重调整) - 调整 P0-P9 优先级权重
2. **Quota Adjustment** (配额调整) - 动态调整账户配额
3. **Capacity Adjustment** (容量调整) - 调整全局容量限制

## 实际使用场景示例

### 场景 1: 工作时间优先内部，下班优先外部

**业务需求**：
- 白天 (8:00-18:00)：内部员工优先使用
- 晚上 (18:00-8:00)：外部客户获得更多资源

**配置示例**：

```ini
[time_rules]
enabled = true
check_interval_sec = 60
default_timezone = Asia/Singapore

# 白天规则：内部优先
[rule.weights.daytime]
type = weight_adjustment
name = 内部优先（白天）
enabled = true
start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri

# P0-P3 是内部部门，给高权重
# P5-P9 是外部客户，给低权重
weights = 256,128,64,32,16,8,4,2,1,1

# 晚上规则：外部优先
[rule.weights.nighttime]
type = weight_adjustment
name = 外部优先（晚上）
enabled = true
start_time = 18:00
end_time = 08:00  # 跨午夜

# 拉平权重，给外部客户更多份额
weights = 32,32,32,32,32,64,64,64,32,16
```

**效果**：
- 白天：内部请求（P0-P3）获得更多 CPU/GPU 时间
- 晚上：外部请求（P5-P9）获得更多资源

---

### 场景 2: 午餐高峰时段扩容

**业务需求**：
- 12:00-14:00 午餐时间使用量激增
- 临时增加容量应对高峰

**配置示例**：

```ini
[rule.capacity.lunch_peak]
type = capacity_adjustment
name = 午餐高峰扩容
description = 增加容量应对午餐时间高峰
enabled = true
start_time = 12:00
end_time = 14:00
days_of_week = Mon,Tue,Wed,Thu,Fri

# 临时增加容量
max_concurrent = 200      # 并发请求数
max_rps = 500            # 每秒请求数
max_tokens_per_sec = 10000  # 每秒 token 数
```

**效果**：
- 12:00-14:00：容量翻倍，可以处理更多请求
- 其他时间：恢复正常容量

---

### 场景 3: 动态配额分配

**业务需求**：
- 工作时间：内部部门需要大配额
- 下班后：外部客户需要更多配额

**配置示例**：

```ini
# 白天配额：内部部门优先
[rule.quota.daytime]
type = quota_adjustment
name = 白天配额分配
enabled = true
start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri

# 内部部门配额（慷慨）
quota.dept-engineering-* = concurrent:50,rps:100,tokens_per_sec:2000
quota.dept-product-* = concurrent:40,rps:80,tokens_per_sec:1500
quota.dept-sales-* = concurrent:30,rps:60,tokens_per_sec:1000

# 外部客户配额（限制）
quota.customer-premium-* = concurrent:10,rps:20,tokens_per_sec:300
quota.customer-standard-* = concurrent:5,rps:10,tokens_per_sec:150
quota.customer-free-* = concurrent:2,rps:5,tokens_per_sec:50

# 晚上配额：外部客户优先
[rule.quota.nighttime]
type = quota_adjustment
name = 晚上配额分配
enabled = true
start_time = 18:00
end_time = 08:00

# 内部部门配额（减少）
quota.dept-engineering-* = concurrent:15,rps:30,tokens_per_sec:500
quota.dept-product-* = concurrent:10,rps:20,tokens_per_sec:300
quota.dept-sales-* = concurrent:10,rps:20,tokens_per_sec:300

# 外部客户配额（扩大）
quota.customer-premium-* = concurrent:40,rps:80,tokens_per_sec:1500
quota.customer-standard-* = concurrent:25,rps:50,tokens_per_sec:800
quota.customer-free-* = concurrent:10,rps:20,tokens_per_sec:200
```

**配额参数说明**：
- `concurrent`: 最大并发请求数
- `rps`: 每秒请求数限制
- `tokens_per_sec`: 每秒 token 数限制

---

### 场景 4: 凌晨省电模式

**业务需求**：
- 凌晨 2:00-6:00 使用量很低
- 降低容量节省成本

**配置示例**：

```ini
[rule.capacity.night_power_save]
type = capacity_adjustment
name = 凌晨省电模式
description = 降低容量节省资源
enabled = true
start_time = 02:00
end_time = 06:00

# 最小容量配置
max_concurrent = 20
max_rps = 50
max_tokens_per_sec = 1000
```

**效果**：
- 凌晨时段：只保留最小容量
- 其他时间：恢复正常容量

---

### 场景 5: 周末平等对待

**业务需求**：
- 周末不区分内部外部
- 所有用户平等对待

**配置示例**：

```ini
[rule.weights.weekend]
type = weight_adjustment
name = 周末平等模式
description = 周末所有优先级平等
enabled = true
start_time = 00:00
end_time = 23:59
days_of_week = Sat,Sun

# 所有优先级相同权重
weights = 32,32,32,32,32,32,32,32,32,32
```

---

### 场景 6: 多时段组合策略

**业务需求**：
- 早上 (06:00-09:00)：渐进扩容
- 白天 (09:00-18:00)：全力运行
- 晚上 (18:00-22:00)：外部优先
- 深夜 (22:00-06:00)：低功耗

**配置示例**：

```ini
# 早上扩容
[rule.capacity.morning_rampup]
type = capacity_adjustment
name = 早上渐进扩容
enabled = true
start_time = 06:00
end_time = 09:00
max_concurrent = 100
max_rps = 300
max_tokens_per_sec = 5000

# 白天全力
[rule.capacity.daytime_full]
type = capacity_adjustment
name = 白天全力运行
enabled = true
start_time = 09:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri
max_concurrent = 200
max_rps = 600
max_tokens_per_sec = 12000

# 晚上外部优先 + 适度容量
[rule.weights.evening]
type = weight_adjustment
name = 晚上外部优先
enabled = true
start_time = 18:00
end_time = 22:00
weights = 16,16,16,16,16,64,64,64,32,32

[rule.capacity.evening]
type = capacity_adjustment
name = 晚上适度容量
enabled = true
start_time = 18:00
end_time = 22:00
max_concurrent = 150
max_rps = 400
max_tokens_per_sec = 8000

# 深夜省电
[rule.capacity.night_minimal]
type = capacity_adjustment
name = 深夜最小容量
enabled = true
start_time = 22:00
end_time = 06:00
max_concurrent = 30
max_rps = 80
max_tokens_per_sec = 1500
```

---

## 配置参数详解

### 时间窗口参数

```ini
start_time = HH:MM        # 开始时间 (24小时制)
end_time = HH:MM          # 结束时间 (可以跨午夜，如 18:00-08:00)
days_of_week = Mon,Tue,Wed,Thu,Fri,Sat,Sun  # 星期过滤（可选）
timezone = Asia/Singapore # 时区（可选，默认用全局时区）
```

### 权重调整参数

```ini
type = weight_adjustment
weights = w0,w1,w2,w3,w4,w5,w6,w7,w8,w9  # P0-P9 的权重值
```

**权重计算**：
- 权重越大 = 获得更多调度时间
- 例如：P0=256, P5=8，则 P0 获得的资源是 P5 的 32 倍

### 配额调整参数

```ini
type = quota_adjustment
quota.{pattern} = concurrent:{num},rps:{num},tokens_per_sec:{num}
```

**通配符模式**：
- `dept-a-*` - 匹配所有以 dept-a- 开头的账户
- `*-premium` - 匹配所有以 -premium 结尾的账户
- `*vip*` - 匹配所有包含 vip 的账户

### 容量调整参数

```ini
type = capacity_adjustment
max_concurrent = {number}      # 最大并发请求数
max_rps = {number}            # 每秒最大请求数
max_tokens_per_sec = {number} # 每秒最大 token 数
```

---

## 运维命令

### 查看规则状态

```bash
# 查看所有规则及其 active 状态
curl http://localhost:8081/admin/time-rules/status \
  -H "Authorization: Bearer <admin-key>" | jq .
```

**返回示例**：
```json
{
  "enabled": true,
  "count": 7,
  "rules": [
    {
      "name": "内部优先（白天）",
      "type": "weight_adjustment",
      "active": true,
      "window": "08:00-18:00 [Mon Tue Wed Thu Fri] (Asia/Singapore)",
      "last_applied": "2025-11-24T10:30:00+08:00"
    },
    {
      "name": "外部优先（晚上）",
      "type": "weight_adjustment",
      "active": false,
      "window": "18:00-08:00 all days (Asia/Singapore)",
      "last_applied": "0001-01-01T00:00:00Z"
    }
  ]
}
```

### 手动触发规则评估

```bash
# 立即评估并应用规则（不等待下一个check interval）
curl -X POST http://localhost:8081/admin/time-rules/apply \
  -H "Authorization: Bearer <admin-key>" | jq .
```

### 重新加载配置

```bash
# 修改配置文件后，重启 gateway
make gfr

# 或者发送信号（如果实现了信号处理）
kill -HUP <gatewayd-pid>
```

---

## 最佳实践

1. **时间窗口不要重叠同类型规则**
   - ❌ 错误：两个 weight_adjustment 规则同时 active
   - ✅ 正确：白天、晚上、周末互不重叠

2. **使用合理的 check_interval**
   - 太短：CPU 占用高
   - 太长：规则切换不及时
   - 推荐：60 秒

3. **先在测试环境验证**
   ```bash
   # 测试模式：5秒检查一次，快速验证
   check_interval_sec = 5
   ```

4. **监控规则应用情况**
   ```bash
   # 查看日志
   tail -f logs/gatewayd.log | grep -i "RuleEngine"
   ```

5. **配额调整要考虑实际容量**
   - 所有账户配额之和不要超过全局容量
   - 预留一些缓冲空间

---

## 故障排查

### 规则没有生效

```bash
# 1. 检查规则是否 enabled
grep "enabled = true" config/scheduler_time_rules.ini

# 2. 检查时间窗口
curl http://localhost:8081/admin/time-rules/status | jq '.rules[] | {name, active, window}'

# 3. 检查日志
grep "RuleEngine" logs/gatewayd.log
```

### 时区问题

```bash
# 检查系统时区
date
TZ=Asia/Singapore date

# 在配置中明确指定时区
[time_rules]
default_timezone = Asia/Singapore
```

### 规则冲突

```bash
# 查看当前哪些规则是 active
curl http://localhost:8081/admin/time-rules/status | \
  jq '.rules[] | select(.active==true) | {name, type}'
```
