# Scheduling System Documentation

This directory contains comprehensive design documentation for Tokligence Gateway's request scheduling, capacity allocation, and routing systems.

## Quick Start

**New to this documentation?** Start here:

1. Read [`00_SCHEDULING_OVERVIEW.md`](./00_SCHEDULING_OVERVIEW.md) - High-level overview and navigation guide
2. Read [`05_ORTHOGONALITY_ANALYSIS.md`](./05_ORTHOGONALITY_ANALYSIS.md) - Understand how components work together
3. Choose your implementation path based on use case

## Document Index

| # | Document | Description | Status |
|---|----------|-------------|--------|
| **00** | [SCHEDULING_OVERVIEW.md](./00_SCHEDULING_OVERVIEW.md) | System overview, navigation, and integration guide | ✅ Complete |
| **01** | [PRIORITY_BASED_SCHEDULING.md](./01_PRIORITY_BASED_SCHEDULING.md) | Traditional P0-P4 priority queue design | ✅ Complete |
| **02** | [BUCKET_BASED_SCHEDULING.md](./02_BUCKET_BASED_SCHEDULING.md) | 100-bucket capacity allocation model | ✅ Complete |
| **03** | [CONFIGURABLE_BUCKET_COUNT.md](./03_CONFIGURABLE_BUCKET_COUNT.md) | Configurable bucket count (2-100) implementation | ✅ Complete |
| **04** | [TOKEN_BASED_ROUTING.md](./04_TOKEN_BASED_ROUTING.md) | Database-driven API token routing with header support | ✅ Complete |
| **05** | [ORTHOGONALITY_ANALYSIS.md](./05_ORTHOGONALITY_ANALYSIS.md) | How all systems integrate (orthogonality proof) | ✅ Complete |

## Reading Paths

### Path 1: For Decision Makers

**Goal:** Understand options and choose the right architecture

1. **Overview** (00) - Understand the landscape
2. **Trade-off Analysis** (05 §7) - Compare approaches
3. **Recommendation** (00 §9) - Choose based on use case

**Time:** 30-45 minutes

### Path 2: For Architects

**Goal:** Understand system design and integration

1. **Overview** (00) - Big picture
2. **Priority-Based** (01 §1-2) - Traditional approach
3. **Bucket-Based** (02 §1-4) - Novel approach
4. **Orthogonality** (05) - Complete integration analysis

**Time:** 2-3 hours

### Path 3: For Implementers

**Goal:** Get implementation-ready designs

1. **Token Routing** (04) - Start here (foundational)
2. **Orthogonality** (05 §5) - Plugin architecture
3. Choose one:
   - **Priority-Based** (01 §3) - Simpler implementation
   - **Bucket-Based** (02 §6 + 03 §3) - More flexible

**Time:** 4-6 hours

### Path 4: For Operators

**Goal:** Configure and deploy

1. **Overview** (00 §9) - Choose scheduler type
2. **Capacity Benchmarking** (02 §2) - Measure your capacity
3. **Configuration** (03 §7) - Best practices and presets

**Time:** 1-2 hours

## Key Concepts

### Three Orthogonal Layers

```
Layer 1: CLASSIFICATION
  │ Who is this request from? What tier?
  ├─ HTTP Header-based (X-TGW-Source)
  ├─ API Token-based (database lookup)
  └─ Hybrid (header → token → default)

Layer 2: ALLOCATION
  │ Where does capacity come from?
  ├─ Priority-based (P0-P4)
  └─ Bucket-based (0-99 capacity buckets)

Layer 3: SCHEDULING
  │ When and how to execute?
  ├─ Strict Priority
  ├─ Weighted Fair Queuing
  ├─ Deficit Round Robin
  └─ AtLeast (opportunistic)
```

Each layer is **independent** - you can swap implementations without affecting others.

## Design Decisions

### Priority vs Bucket

| Scenario | Recommended | Why |
|----------|-------------|-----|
| Small deployment (< 100 RPS) | **Priority-based** (01) | Simple, proven, sufficient |
| Medium deployment (100-1000 RPS) | **10-Bucket model** (02+03) | Fine-grained, concrete capacity |
| Large enterprise (1000+ RPS) | **20-30 Bucket model** (03) | Very fine-grained control |
| Multi-tenant SaaS (many customers) | **50-100 Bucket model** (02+03) | Precise per-tier SLAs |

### Classification Method

| Scenario | Recommended | Why |
|----------|-------------|-----|
| Single gateway | **Token-based** (04 §3) | Fine-grained, billing-friendly |
| Multi-gateway architecture | **Header-based** (04 §1.3, §6.3) | Fast, no DB lookup |
| Hybrid deployment | **Hybrid** (04 §1.3) | Flexible, supports both |

## Quick Reference

### Default Configuration (Recommended)

```ini
# config/gateway.ini

[request_scheduler]
classifier = hybrid      # Header → Token → Default
allocator = bucket       # Bucket-based capacity
scheduler = atleast      # Opportunistic scheduling

[allocator.bucket]
bucket_count = 10        # Default: 10 buckets
base_capacity_rps = 100  # From: tokligence benchmark
decay_ratio = 0.7

[classifier.header]
enabled = true
trusted_cidrs = 10.0.0.0/8
```

### Migration Commands

```bash
# Benchmark your LLM capacity
tokligence benchmark --duration 300s --output report.json

# Configure scheduler interactively
tokligence scheduler configure

# Migrate from priority to bucket
tokligence migrate-to-bucket-scheduler --bucket-count 10

# Validate configuration
tokligence validate-config

# Visualize bucket distribution
tokligence bucket-scheduler visualize
```

## Implementation Status

| Component | Status | Document |
|-----------|--------|----------|
| Token Classification | 📝 Design Complete | 04 |
| Header Classification | 📝 Design Complete | 04 §6.3 |
| Priority Allocator | 📝 Design Complete | 01 |
| Bucket Allocator | 📝 Design Complete | 02, 03 |
| Strict Scheduler | 📝 Design Complete | 01 §3.3 |
| WFQ Scheduler | 📝 Design Complete | 01 §3.3 |
| AtLeast Scheduler | 📝 Design Complete | 02 §5.2 |
| Capacity Guard | 📝 Design Complete | 04 §5 |
| Time Windows | 📝 Design Complete | 01 §1.3 |
| Benchmarking Tool | 📝 Design Complete | 02 §2 |

**Legend:**
- 📝 Design Complete
- 🚧 In Progress
- ✅ Implemented
- ❌ Not Started

## Related Documentation

### In This Repository

- [`../CLAUDE.md`](../CLAUDE.md) - Project overview and development guidelines
- [`../QUICK_START.md`](../QUICK_START.md) - Getting started guide
- [`../codex-to-anthropic.md`](../codex-to-anthropic.md) - Codex CLI integration
- [`../claude_code-to-openai.md`](../claude_code-to-openai.md) - Claude Code integration

### Architecture Documentation

- [`../../arc_design/00_ARCHITECTURE_INDEX.md`](../../arc_design/00_ARCHITECTURE_INDEX.md) - Overall system architecture
- [`../../arc_design/01_comprehensive_system_architecture.md`](../../arc_design/01_comprehensive_system_architecture.md) - Detailed architecture
- [`../../arc_design/04_trading_and_matching_engine.md`](../../arc_design/04_trading_and_matching_engine.md) - Trading system

### External References

- [LiteLLM Scheduler](https://docs.litellm.ai/docs/scheduler) - Industry reference
- [Kubernetes Pod Priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) - Priority scheduling patterns
- [NGINX Rate Limiting](https://www.nginx.com/blog/rate-limiting-nginx/) - Request queuing patterns

## Contributing

When adding new scheduling designs:

1. Create new document with sequential number (06, 07, etc.)
2. Update this README with document index entry
3. Update `00_SCHEDULING_OVERVIEW.md` if adding new layer/component
4. Update `05_ORTHOGONALITY_ANALYSIS.md` if new component interacts with existing layers

## Feedback

- **Issues:** https://github.com/anthropics/claude-code/issues
- **Discussions:** For design questions or suggestions

---

**Last Updated:** 2025-02-01
