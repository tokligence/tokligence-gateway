# Scheduling 文档评审（实用性视角）

## 总体印象
- 文档覆盖度很高，已经将分类（header/token）、分配（priority/bucket）、执行（多种调度算法）拆成正交层，并给了迁移路线和默认配置。
- 设计目标对“自建 LLM 卖容量/买容量”场景有意识，但细节仍偏向单网关内部调度；与 marketplace（供给发现、定价、结算）的结合还不够落地。
- Bucket 与 Priority 两条线并行设计清晰，但工程落地复杂度和观测/运维成本在文档里被低估。

## 主要问题与风险
- 业务契合度：卖容量场景缺少供给侧能力（供给可用性探测、动态报价、自动下架/降配、健康度回传），当前设计更多是“单网关内部分层调度”，尚未说明如何把多家 self-hosted 供给挂上 marketplace 并做撮合或跨网关路由。
- 资源度量模型偏 RPS：bucket capacity 以 RPS 为主，未结合 token 速率、上下文长度、模型差异（chat vs embedding）、streaming/工具调用等因素；这会导致 SLA 表达与真实吞吐（tokens/sec、上下文占用）脱节，卖容量的计费/履约也难校验。
- 100-bucket 方案落地性：文档已自评“过度复杂”，但仍保留 100 桶 + hybrid decay 作为主叙事，会导致配置、测试、Prometheus 指标基数和告警规则膨胀；在多模型、多实例场景下，按 bucket 粒度暴露 metrics/alerts 容易爆表。
- AtLeast 模式公平性：低优先级请求可持续占用高优先级空闲，缺少“升级配额”或“可被踢回原桶”的约束，SLA 解释空间大；性能分析假设单模型单实例，未覆盖高并发下 bitmap/锁竞争成本。
- Token 路由依赖链风险：请求路径依赖 LRU → Redis → Postgres；未设计 Postgres/Redis 不可用时的降级（例如只读缓存、静态 allowlist、限流 fail-open/close 策略），可能把网关变成 SPOF。
- Header 路由安全性：仅靠 CIDR 信任过于脆弱，缺少 mTLS / HMAC 签名或 gateway-to-gateway 认证，容易被同网段滥用；并且 header 与 token 双路径的审计/计费一致性未说明。
- 配额与预估：enqueue 前用“estimated tokens”做预扣，未给出估算误差的 buffer 或回滚流程，streaming/工具调用的最终 tokens 可能大于预估，存在透支或误杀风险。
- 容量保护模型单一：固定“internal >=90% 拒外部”缺少 per-model、per-cluster、per-region 的水位配置，也未覆盖 GPU 持续占用型 workload（长上下文、长流）。
- 观测与告警：指标维度（priority、bucket、env、tenant）相乘后在 50+/100 bucket 下会导致高基数，文档没有给出约束或降维方案；也缺少对 tail latency、排队时间分布、升级/降级事件的可视化建议。
- 客户体验/预emption：提出可杀低优先级流式请求，但未描述客户端提示、重试建议或对账（被切断的 tokens 是否计费）。

## 可改进方向
- 需求聚焦：将“默认路径”收敛为 5-level priority 或 10-bucket；100-bucket 留在实验分支，文档首页明确不建议生产使用。
- 度量与配额：以 tokens/sec + 并发 as primary，RPS 作为辅值；按模型/实例生成 capacity profile，并在调度决策中携带模型维度，避免单一 bucket 适配多模型。
- AtLeast 约束：给低优先级升级次数/速率上限，或在高优先级活跃时把已升级的低优先级请求逐步回落（软抢占），并为升级事件产生日志/指标。
- 可用性设计：为 token 路由引入只读缓存与“安全降级”模式（DB 挂掉时允许现有 token 短时续用、严格限流外部新 token）；CapacityGuard 应支持 per-model/per-region 配置。
- 安全与结算：Header 路由增加 mTLS/签名校验，并记录 header 决策的审计事件；明确“header 路由 + token 计费”时的失败模式（缺 token 是否拒绝）。
- 配置与观测：对 metrics 维度做 hard cap（例如 bucket_count>20 时只暴露区间指标），提供默认 Grafana 看板示例；将告警侧重在 queue_wait_p99、upgrade_ratio、capacity_guard 触发率，而非全桶深度。
- Marketplace 对接：补充供给端注册/下架、健康探测、价格/容量动态发布、和撮合/计费对账的接口或时序图，说明 scheduler 如何与上层匹配/竞价逻辑协作。
- 测试策略：在性能评估中加入多模型、多实例、长连接/流式请求场景的基准，覆盖 Redis/DB 故障注入和容量保护切换测试。

## 结论
- 文档奠定了分层与正交组合的基础，但生产落地仍应以“5-level priority → 10-bucket”作为主线，先解决容量度量、容灾、观测与安全，再讨论更细粒度的 bucket/算法。
- 若目标是支撑 marketplace 卖容量，需补齐供给编排与计费一致性设计，否则调度能力难以直接转化为可售 SLA。
