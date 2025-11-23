# Tokligence Gateway Web UI Edition Strategy

**Version**: v2.0 (Marketplace-Focused Revision)
**Date**: 2025-11-22
**Author**: Product Planning
**Status**: Planning Document

---

## Executive Summary

This document outlines the Web UI strategy for three editions of Tokligence Gateway: **Personal**, **Team**, and **Enterprise**. Tokligence is a **dual-sided token marketplace** where users can both **consume** (buy) and **supply** (sell) AI tokens, similar to OpenRouter but with provider monetization capabilities.

### Key Differentiators vs LiteLLM

| Feature | LiteLLM | Tokligence Gateway |
|---------|---------|-------------------|
| **Token Direction** | Consume only | Consume + Supply (dual-sided) |
| **Marketplace** | No marketplace | Full token marketplace with pricing |
| **Provider Monetization** | Not supported | Providers can sell tokens and earn revenue |
| **Settlement** | N/A | Automatic cost calculation and settlement |
| **Role Model** | Single consumer | Consumer, Provider, or both |

### Core Design Principles

1. **Marketplace-First**: ALL editions have marketplace enabled by default (traffic/revenue critical)
2. **Consumer-First UX**: Default interface optimized for token buyers (most common use case)
3. **Provider Revenue Visibility**: Clear earning dashboards for token sellers (platform revenue driver)
4. **Dual-Role Flexibility**: Users can be consumers, providers, or both simultaneously
5. **Transparent Pricing**: Real-time cost calculation and settlement visibility
6. **Progressive Complexity**: Personal = basic marketplace; Team = provider features; Enterprise = governance

---

## Business Model Context

### Revenue Streams

1. **Marketplace Transaction Fees** (Primary)
   - Take rate on provider sales (e.g., 10-15% of token sales)
   - Volume-based pricing tiers
   - Settlement processing fees

2. **Subscription Tiers** (Secondary)
   - Personal: Free (marketplace consumer access only)
   - Team: Monthly subscription (marketplace consumer + provider features)
   - Enterprise: Custom pricing (full marketplace + white-label + custom integrations)

3. **Premium Features** (Tertiary)
   - Advanced analytics
   - Priority support
   - Custom SLA guarantees

### User Journey Flow

```
Consumer Journey:
1. Sign up → Browse marketplace → Subscribe to providers
2. Use gateway endpoint → Consume tokens → Pay marketplace
3. Track spending → Optimize costs → Discover cheaper providers

Provider Journey:
1. Sign up → Register as provider → Publish service offerings
2. Set pricing → List on marketplace → Earn from consumers
3. Track revenue → Optimize pricing → Scale capacity
```

---

## Edition Comparison Matrix

| Feature Category | Personal | Team | Enterprise |
|-----------------|----------|------|------------|
| **Authentication** | Optional (default off) | Required | Required + SSO |
| **User Management** | Single user | Multi-user + Roles | Advanced RBAC + Teams |
| **API Key Management** | Self-service only | Admin can manage all | Scoped + Approval workflows |
| **Marketplace Access** | **✅ Consumer only (browse/buy)** | **✅ Full (consumer + provider)** | **✅ Full + White-label** |
| **Consumer Features** | Browse + Subscribe | Browse + Subscribe + Team analytics | Advanced analytics + Custom pricing |
| **Provider Features** | **🔒 Locked (upgrade prompt)** | **✅ Publish + Earn** | **✅ Custom pricing + Revenue analytics** |
| **Revenue Dashboard** | N/A (consumer only) | Basic provider earnings | Advanced revenue analytics |
| **Settlement** | Pay-as-you-go (consumer) | Consumer billing + Provider payouts | Custom payment terms + Net settlement |
| **Usage Dashboard** | Basic consumption metrics | Team aggregation (consume + supply) | Advanced analytics + Reports |
| **Configuration** | Simple form | Grouped settings | Environment management |
| **Monitoring** | Basic logs | Real-time metrics | APM + Alerting |
| **Audit Logs** | N/A | Basic activity | Comprehensive audit trail |
| **Support** | Community | Email support | SLA + Dedicated support |

---

## Dual-Sided Marketplace: Core UI Patterns

### Consumer vs Provider Modes

All Team and Enterprise users have a **role switcher** that changes the interface context:

```
┌─────────────────────────────────────────┐
│ Tokligence Gateway                      │
│                                         │
│ Mode: [👤 Consumer ▼]                   │
│       - Consumer (default)              │
│       - Provider (if enabled)           │
│       - Both (split view)               │
└─────────────────────────────────────────┘
```

**Consumer Mode** (Default - 90% of users):
- Browse marketplace providers
- Subscribe to token services
- Track spending and token consumption
- Optimize costs across providers
- See balance: "You owe marketplace: $142.30"

**Provider Mode** (Revenue-generating users):
- Publish token service offerings
- Set pricing per 1K tokens
- Track revenue and token supply
- View customer analytics
- See balance: "Marketplace owes you: $1,240.50"

**Both Mode** (Power users):
- Split dashboard showing both consume and supply
- Net balance calculation (revenue - spending)
- Cross-role analytics

---

## Marketplace Dashboard (Team & Enterprise)

### Consumer View (Default)

```
┌─────────────────────────────────────────┐
│ 🛒 Marketplace - Buy Tokens             │
├─────────────────────────────────────────┤
│ Your Spending This Month                │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ Total    │ │ Consumed │ │ Active   │ │
│ │ Cost     │ │ Tokens   │ │ Services │ │
│ │ $142.30  │ │  450K    │ │    5     │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ ⚡ Browse Token Marketplace             │
│ Search: [____________]  Sort: [Price ▼]│
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ GPT-4o                             │ │
│ │ OpenAI Official                    │ │
│ │ $0.0250/1K tokens                  │ │
│ │ ⭐ 4.9 (2.3K reviews)              │ │
│ │ [Subscribe] [Try free 10K tokens] │ │
│ └────────────────────────────────────┘ │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ Claude 3.5 Sonnet                  │ │
│ │ Anthropic Verified                 │ │
│ │ $0.0150/1K tokens  💎 Premium      │ │
│ │ ⭐ 4.8 (1.8K reviews)              │ │
│ │ [Subscribed ✓] [Manage]           │ │
│ └────────────────────────────────────┘ │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ GPT-4o Budget                      │ │
│ │ ThirdParty GPU Farm                │ │
│ │ $0.0180/1K tokens  🔥 20% cheaper  │ │
│ │ ⭐ 4.2 (342 reviews)               │ │
│ │ [Subscribe] [Compare pricing]      │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Your Subscribed Services (5)            │
│ [Manage subscriptions] [Usage breakdown]│
└─────────────────────────────────────────┘
```

**Key Features**:
- **Pricing Transparency**: Show price per 1K tokens prominently
- **Social Proof**: Ratings, reviews, subscriber counts
- **Cost Comparison**: Highlight savings vs official APIs
- **Free Trials**: Encourage first-time subscriptions
- **Trust Indicators**: Verified badges, uptime stats

### Provider View (Revenue Dashboard)

```
┌─────────────────────────────────────────┐
│ 💰 Marketplace - Sell Tokens            │
├─────────────────────────────────────────┤
│ Your Revenue This Month                 │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ Gross    │ │ Supplied │ │ Active   │ │
│ │ Revenue  │ │ Tokens   │ │ Customers│ │
│ │ $1,420   │ │  2.8M    │ │   142    │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ Platform │ │ Net      │ │ Next     │ │
│ │ Fee (10%)│ │ Earnings │ │ Payout   │ │
│ │ -$142    │ │ $1,278   │ │ Dec 1    │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ Your Published Services       [+ Publish]│
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ My GPT-4o Service                  │ │
│ │ Status: ✅ Active                   │ │
│ │ Price: $0.0220/1K  Margin: 12%    │ │
│ │ Revenue: $890 (134 customers)      │ │
│ │ Rating: ⭐ 4.6 (89 reviews)        │ │
│ │ [Edit] [Pause] [Analytics]         │ │
│ └────────────────────────────────────┘ │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ My Claude Sonnet API               │ │
│ │ Status: ⏸️ Paused (Low inventory)   │ │
│ │ Price: $0.0140/1K  Margin: 8%     │ │
│ │ Revenue: $530 (73 customers)       │ │
│ │ [Resume] [Adjust pricing]          │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Revenue Trends (Last 30 days)           │
│ [Chart: Daily revenue + token supply]  │
│                                         │
│ Top Customers by Volume                 │
│ 1. customer_abc: $142 (45K tokens)     │
│ 2. customer_xyz: $98 (32K tokens)      │
│ 3. customer_def: $76 (28K tokens)      │
└─────────────────────────────────────────┘
```

**Key Features**:
- **Revenue Visibility**: Gross, fees, net earnings front and center
- **Service Performance**: Per-service revenue and customer counts
- **Pricing Control**: Easy price adjustments with margin calculator
- **Customer Insights**: See who's consuming your tokens
- **Payout Schedule**: Clear next payment date and amount
- **Inventory Management**: Pause/resume services based on capacity

### Publish Service Flow (Provider Onboarding)

**Step 1: Service Details**
```
┌──────────────────────────────┐
│ Publish New Token Service    │
├──────────────────────────────┤
│ Service Name:                │
│ [My GPT-4o Proxy________]    │
│                              │
│ Model Family:                │
│ [gpt-4o ▼]                   │
│                              │
│ Description:                 │
│ [High-performance GPT-4o     │
│  with 99.9% uptime and       │
│  dedicated GPU cluster____]  │
│                              │
│ [Cancel]     [Next: Pricing] │
└──────────────────────────────┘
```

**Step 2: Pricing Strategy**
```
┌──────────────────────────────┐
│ Set Your Pricing             │
├──────────────────────────────┤
│ Price per 1K tokens:         │
│ $ [0.0220______]             │
│                              │
│ 💡 Pricing Guidance:         │
│ • OpenAI official: $0.0250   │
│ • Marketplace avg: $0.0210   │
│ • Suggested: $0.0200-$0.0230 │
│                              │
│ Your Cost: $0.0196/1K        │
│ Your Margin: 12% ($0.0024)   │
│ Platform Fee: 10% ($0.0022)  │
│ Net per 1K: $0.0198          │
│                              │
│ Trial Tokens (optional):     │
│ [10000__] free tokens/user   │
│                              │
│ [Back]      [Next: Capacity] │
└──────────────────────────────┘
```

**Step 3: Capacity & Limits**
```
┌──────────────────────────────┐
│ Service Capacity             │
├──────────────────────────────┤
│ Max requests/minute:         │
│ [1000____]                   │
│                              │
│ Max tokens/day:              │
│ [10000000] (10M)             │
│                              │
│ Auto-pause when:             │
│ ☑ Upstream API fails         │
│ ☑ Daily quota exceeded       │
│ ☐ Cost exceeds $100/day      │
│                              │
│ [Back]      [Publish Service]│
└──────────────────────────────┘
```

**Step 4: Confirmation**
```
┌──────────────────────────────┐
│ ✅ Service Published!         │
├──────────────────────────────┤
│ Your service is now live on  │
│ Tokligence Marketplace.      │
│                              │
│ Service ID: #12345           │
│ Endpoint: gw.tokligence.ai   │
│                              │
│ What's next:                 │
│ • Share your service URL     │
│ • Monitor first customers    │
│ • Adjust pricing if needed   │
│                              │
│ [View Service] [Share] [Done]│
└──────────────────────────────┘
```

---

## Settlement & Payment UI

### Consumer Settlement

```
┌─────────────────────────────────────────┐
│ 💳 Billing & Settlement                 │
├─────────────────────────────────────────┤
│ Current Balance                         │
│ You owe marketplace: $142.30            │
│                                         │
│ Billing Cycle: Nov 1 - Nov 30          │
│ Payment Due: Dec 1, 2025                │
│                                         │
│ Usage Breakdown by Service              │
│ ┌────────────────────────────────────┐ │
│ │ Service         Tokens    Cost     │ │
│ ├────────────────────────────────────┤ │
│ │ GPT-4o Official  180K    $45.00   │ │
│ │ Claude Sonnet    240K    $36.00   │ │
│ │ GPT-4o Budget    150K    $27.00   │ │
│ │ Llama 3 70B       80K     $1.20   │ │
│ │ Platform Fee              $10.92   │ │
│ │ Sales Tax (10%)           $12.01   │ │
│ ├────────────────────────────────────┤ │
│ │ Total Due                $142.30   │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Payment Method                          │
│ 💳 Visa •••• 4242                       │
│ [Change payment method]                 │
│                                         │
│ [Pay Now $142.30] [Download Invoice]    │
└─────────────────────────────────────────┘
```

### Provider Settlement

```
┌─────────────────────────────────────────┐
│ 💰 Earnings & Payouts                   │
├─────────────────────────────────────────┤
│ Available Balance                       │
│ Marketplace owes you: $1,278.00         │
│                                         │
│ Next Payout: Dec 1, 2025                │
│ Estimated: $1,278.00                    │
│                                         │
│ Revenue Breakdown by Service            │
│ ┌────────────────────────────────────┐ │
│ │ Service         Tokens    Revenue  │ │
│ ├────────────────────────────────────┤ │
│ │ My GPT-4o      1.8M      $890.00  │ │
│ │ My Claude API  1.0M      $530.00  │ │
│ ├────────────────────────────────────┤ │
│ │ Gross Revenue           $1,420.00  │ │
│ │ Platform Fee (10%)       -$142.00  │ │
│ ├────────────────────────────────────┤ │
│ │ Net Earnings            $1,278.00  │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Payout Method                           │
│ 🏦 Bank •••• 6789                       │
│ [Change payout method]                  │
│                                         │
│ Payout History                          │
│ Nov 2025: $1,142.30 (Paid ✅)           │
│ Oct 2025: $987.45 (Paid ✅)             │
│ [View all payouts]                      │
└─────────────────────────────────────────┘
```

---

## 1. Personal Edition UI Strategy

### Target Users
- Individual developers
- Solo researchers
- Students and educators
- Testing and prototyping
- First-time marketplace consumers

### Key Characteristics
- **Zero-config philosophy**: Works out of the box
- **No authentication overhead**: Direct access, minimal barriers (can use marketplace as guest)
- **Focus on speed**: Quick setup and experimentation
- **Marketplace consumer access**: ✅ Can browse and subscribe to providers (buy tokens)
- **Provider features locked**: 🔒 Cannot publish services or earn revenue (upgrade to Team)
- **Gateway to monetization**: Strategic upgrade path to Team edition when users want to become providers

### UI Components

#### 1.1 Dashboard (Marketplace-Enabled)

**Layout**: Single-column, card-based with marketplace discovery

```
┌─────────────────────────────────────────┐
│ 🏠 Tokligence Gateway (Personal)        │
├─────────────────────────────────────────┤
│                                         │
│  Quick Start                            │
│  ┌─────────────────────────────────┐   │
│  │ Your Gateway Endpoint           │   │
│  │ http://localhost:8081           │   │
│  │ [Copy]                          │   │
│  └─────────────────────────────────┘   │
│                                         │
│  ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │ Requests │ │  Tokens  │ │  Cost  │ │
│  │   142    │ │   45.2K  │ │ $12.30 │ │
│  └──────────┘ └──────────┘ └────────┘ │
│                                         │
│  🛒 Marketplace - Featured Providers    │
│  ┌────────────────────────────────────┐│
│  │ GPT-4o Budget                      ││
│  │ $0.0210/1K tokens 💰 Save 16%     ││
│  │ ⭐ 4.7 (1.2K reviews)              ││
│  │ [Subscribe] [Try 10K free]        ││
│  └────────────────────────────────────┘│
│  ┌────────────────────────────────────┐│
│  │ Claude 3.5 Sonnet Pro              ││
│  │ $0.0145/1K tokens 🔥 Best price   ││
│  │ ⭐ 4.9 (2.3K reviews)              ││
│  │ [Subscribe]                        ││
│  └────────────────────────────────────┘│
│  [Browse all providers →]              │
│                                         │
│  💡 Become a Provider                  │
│  ┌─────────────────────────────────┐   │
│  │ Got GPUs? Earn $500-$2,000/month│   │
│  │ selling tokens on marketplace.  │   │
│  │ [Upgrade to Team →] [Learn more]│   │
│  └─────────────────────────────────┘   │
│                                         │
│  Recent Activity                        │
│  ├─ gpt-4o (marketplace): 12 req ($0.25)│
│  ├─ claude-sonnet (own API): 8 req    │
│  └─ View all →                          │
└─────────────────────────────────────────┘
```

**Features**:
- **Quick Copy Endpoint**: One-click copy of gateway URL
- **Real-time Stats**: Request count, token usage, **cost tracking**
- **Marketplace Discovery**: Featured providers with pricing (top 2-3)
- **Social Proof**: Ratings and reviews prominently displayed
- **Become Provider CTA**: Strategic upgrade prompt to Team edition
- **Hybrid Usage**: Shows both marketplace and own API usage
- **Navigation**: Dashboard, Marketplace, Settings, Docs

#### 1.2 Marketplace Page (NEW - Consumer Only)

**Full Marketplace Browser**:

```
┌─────────────────────────────────────────┐
│ 🛒 Token Marketplace                    │
├─────────────────────────────────────────┤
│ Search: [____________]  Sort: [Price ▼]│
│ Filter: [All Models ▼] [All Ratings ▼] │
│                                         │
│ 🔥 Featured Providers                   │
│ ┌────────────────────────────────────┐ │
│ │ GPT-4o Official                    │ │
│ │ OpenAI Verified ✓                  │ │
│ │ $0.0250/1K tokens                  │ │
│ │ ⭐ 4.9 (2.3K reviews) 99.9% uptime │ │
│ │ [Subscribe] [Details]              │ │
│ └────────────────────────────────────┘ │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ GPT-4o Budget                      │ │
│ │ ThirdParty GPU Farm                │ │
│ │ $0.0210/1K 💰 16% cheaper          │ │
│ │ ⭐ 4.7 (1.2K reviews) 99.5% uptime │ │
│ │ [Subscribe] [Try 10K free tokens]  │ │
│ └────────────────────────────────────┘ │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ Claude 3.5 Sonnet Pro              │ │
│ │ Anthropic Partner 🏅               │ │
│ │ $0.0145/1K 🔥 Best price           │ │
│ │ ⭐ 4.9 (2.3K reviews) 99.8% uptime │ │
│ │ [Subscribed ✓] [Manage]            │ │
│ └────────────────────────────────────┘ │
│                                         │
│ [Load more providers...]                │
│                                         │
│ 💼 Want to sell tokens?                │
│ ┌─────────────────────────────────┐   │
│ │ 🔒 Provider features locked      │   │
│ │ Upgrade to Team edition to:     │   │
│ │ • Publish your own services     │   │
│ │ • Earn $500-$2,000/month        │   │
│ │ • Access provider analytics     │   │
│ │ [Upgrade to Team $29/mo →]      │   │
│ └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

**Features**:
- **Full provider catalog**: Browse all marketplace offerings
- **Search & Filter**: Find cheapest/best providers
- **Social proof**: Reviews, ratings, uptime stats
- **Free trials**: Encourage first subscription
- **Subscription management**: View/manage active subscriptions
- **Provider upsell**: Prominent "become provider" CTA with locked features
- **Guest access**: Can browse without login, but need account to subscribe

#### 1.3 Settings Page

**Sections**:

1. **Marketplace** (NEW - Top section)
   - Active subscriptions count
   - Total spending this month
   - Payment method on file
   - [Manage subscriptions] button → goes to Marketplace page

2. **API Keys** (Collapsible)
   - OpenAI API Key (password field + test connection)
   - Anthropic API Key (password field + test connection)
   - Gemini API Key (password field + test connection)
   - Status indicators: ✓ Connected / ○ Not configured / ⚠ Error
   - Note: "Using marketplace? You don't need to add your own API keys!"

3. **Model Routing** (Simple toggle)
   - Auto-routing (default: prefer marketplace)
   - Routing priority:
     - ☑ Prefer marketplace providers (recommended)
     - ☐ Use own API keys only
   - Fallback behavior when marketplace unavailable

4. **Advanced** (Hidden by default, expandable)
   - Log level (dropdown: Error, Warn, Info, Debug)
   - Max tokens override
   - Custom base URLs (for proxies)
   - Work mode: Auto / Passthrough / Translation

5. **About**
   - Version info
   - License type: Personal Edition
   - Upgrade to Team prompt (with benefits)

**UX Notes**:
- All changes auto-save after 1 second delay
- Test buttons next to API keys for immediate validation
- Tooltips on hover for every setting
- No "Save" button needed (auto-persist to config file)

#### 1.4 Activity Log (Simple Table with Cost)

**Columns**:
- Timestamp (relative: "2 minutes ago")
- Model
- Source (Marketplace / Own API)
- Tokens (prompt + completion)
- Cost (if marketplace)
- Status (✓ Success / ⚠ Error)

**Features**:
- Last 100 requests only (no pagination)
- Simple search box (filter by model name or source)
- Filter by source: All / Marketplace / Own API
- Export to CSV button
- Auto-refresh every 10 seconds (toggle)
- **Cost tracking**: Shows cumulative spending from marketplace

#### 1.5 Locked/Upgrade Prompts

These features are **visible but locked** in Personal Edition (encourage upgrade):

**🔒 Provider Dashboard** (locked):
```
┌─────────────────────────────────────────┐
│ 💰 Become a Provider                    │
├─────────────────────────────────────────┤
│ 🔒 This feature requires Team edition   │
│                                         │
│ Unlock provider features to:            │
│ • Publish token services                │
│ • Earn $500-$2,000/month                │
│ • Access revenue analytics              │
│ • Set your own pricing                  │
│                                         │
│ Based on marketplace data, providers    │
│ with your usage pattern earn ~$850/mo   │
│                                         │
│ [Upgrade to Team $29/mo →]              │
│ [Learn more about becoming a provider]  │
└─────────────────────────────────────────┘
```

**Removed/Hidden Features**:
- ❌ User management (not applicable for single user)
- ❌ Role/permission settings
- ❌ Team collaboration features
- ❌ Audit logs
- ❌ Advanced analytics (basic only)

---

## 2. Team Edition UI Strategy

### Target Users
- Development teams (5-50 members)
- Small to medium organizations
- Agencies and consultancies
- Multi-project environments

### Key Characteristics
- **Collaboration-first**: Shared resources with role-based access
- **Admin controls**: Centralized management without complexity
- **Cost allocation**: Track usage by user/project

### UI Components

#### 2.1 Enhanced Navigation

```
┌─────────────────────────────────────────┐
│ 🏠 Dashboard  👥 Users  🔑 API Keys     │
│ 📊 Analytics  ⚙️ Settings  🛒 Marketplace│
│                                         │
│ [Profile: admin@company.com ▼]         │
│   - My API Keys                         │
│   - Team Settings                       │
│   - Admin Panel (if root_admin)        │
│   - Logout                              │
└─────────────────────────────────────────┘
```

**Navigation Structure**:
- Dashboard (overview)
- Users (admin only)
- API Keys (self + admin view)
- Analytics (usage breakdown)
- Settings (team configuration)
- Marketplace (optional)

#### 2.2 Dashboard (Team View)

```
┌─────────────────────────────────────────┐
│ Team Usage Overview                     │
│                                         │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ Total    │ │ This     │ │ Active   │ │
│ │ Requests │ │ Month    │ │ Users    │ │
│ │  12.5K   │ │ $142.30  │ │   12/25  │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ Usage by User (Top 5)                   │
│ ├─ alice@team.com      45% ($63.22)    │
│ ├─ bob@team.com        28% ($39.44)    │
│ ├─ charlie@team.com    15% ($21.34)    │
│ └─ View full breakdown →                │
│                                         │
│ Model Distribution                      │
│ [Chart: pie/bar showing gpt-4o 60%,    │
│  claude-sonnet 30%, others 10%]        │
│                                         │
│ Recent Team Activity                    │
│ [Filterable table: User, Model, Time]  │
└─────────────────────────────────────────┘
```

**Features**:
- **Aggregated Metrics**: Team-wide token consumption and costs
- **User Breakdown**: See who's using what (privacy-aware)
- **Model Analytics**: Understand cost drivers
- **Date Range Selector**: Last 7/30/90 days
- **Cost Estimation**: Based on provider pricing

#### 2.3 User Management

**Admin View** (for root_admin and gateway_admin roles):

```
┌─────────────────────────────────────────┐
│ Users & Roles                [+ Add User]│
├─────────────────────────────────────────┤
│ Search: [____________]  Filters: [All ▼]│
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ Email             Role      Status  │ │
│ ├────────────────────────────────────┤ │
│ │ admin@team.com    Admin     Active │ │
│ │ alice@team.com    User      Active │ │
│ │ bob@team.com      User      Active │ │
│ │ charlie@team.com  User      Inactive│ │
│ └────────────────────────────────────┘ │
│                                         │
│ [Bulk Actions: Deactivate, Export CSV] │
└─────────────────────────────────────────┘
```

**User Detail Panel** (slide-out):

```
┌───────────────────────────┐
│ alice@team.com           │
│                          │
│ Role: [gateway_user ▼]  │
│ Status: [Active ▼]      │
│ Display: [Alice Johnson]│
│                          │
│ API Keys (2)             │
│ ├─ key_abc...xyz (30d)  │
│ └─ key_def...uvw (90d)  │
│ [+ Generate new key]     │
│                          │
│ Usage This Month         │
│ - Requests: 1,234       │
│ - Tokens: 45.6K         │
│ - Cost: $23.45          │
│                          │
│ [Save Changes] [Delete] │
└───────────────────────────┘
```

**Features**:
- Create users with email/role/display name
- Assign roles: `gateway_user` or `gateway_admin`
- Activate/deactivate accounts
- Reset passwords (send email verification)
- View per-user API keys and usage
- Bulk import from CSV

#### 2.4 API Key Management

**Two Views**:

1. **My API Keys** (all users)
   - Self-service key generation
   - Set expiration (7d, 30d, 90d, 1y, never)
   - Optional scopes (read-only, full-access)
   - Revoke own keys
   - Copy to clipboard with masked display

2. **Team API Keys** (admins only)
   - View all keys across all users
   - Filter by user/status/expiration
   - Force revoke any key
   - Audit: Created by, Last used

**Key Creation Dialog**:

```
┌──────────────────────────────┐
│ Generate API Key             │
├──────────────────────────────┤
│ For user: alice@team.com     │
│                              │
│ Expiration:                  │
│ ○ 30 days (recommended)      │
│ ○ 90 days                    │
│ ○ 1 year                     │
│ ○ Never (not recommended)    │
│                              │
│ Scopes: (optional)           │
│ ☑ Read access                │
│ ☑ Write access               │
│ ☐ Admin actions              │
│                              │
│ [Cancel]    [Generate Key]   │
└──────────────────────────────┘

┌──────────────────────────────┐
│ ✓ API Key Created            │
├──────────────────────────────┤
│ Save this key securely.      │
│ It won't be shown again.     │
│                              │
│ sk-ant-tokl-abc123...xyz789  │
│ [Copy to Clipboard]          │
│                              │
│ [Close]                      │
└──────────────────────────────┘
```

#### 2.5 Analytics Dashboard

**Sections**:

1. **Time Series Chart**
   - Request volume over time
   - Token consumption over time
   - Cost projection
   - Toggleable model breakdown (stacked area)

2. **Cost Analysis**
   - By user (table + pie chart)
   - By model (table + bar chart)
   - By project (if tags enabled)
   - Export to PDF/CSV

3. **Performance Metrics**
   - Average latency by model
   - P95/P99 latency
   - Error rate
   - Success rate

4. **Alerts & Recommendations** (future)
   - Unusual spending patterns
   - Underutilized API keys
   - Cost-saving suggestions

#### 2.6 Settings (Team Configuration)

**Organized Tabs**:

1. **General**
   - Team name
   - Display name
   - Time zone
   - Default language

2. **Providers**
   - OpenAI API Key (team-wide)
   - Anthropic API Key (team-wide)
   - Gemini API Key (team-wide)
   - Custom provider endpoints
   - Test connection buttons

3. **Routing**
   - Model-to-provider mapping
   - Fallback behavior
   - Custom aliases
   - Work mode (auto/passthrough/translation)

4. **Limits & Quotas**
   - Per-user token limits
   - Per-model rate limits
   - Daily spending caps
   - Alert thresholds

5. **Security**
   - Session timeout
   - Password policy (future: SSO)
   - IP allowlist (optional)
   - Webhook secrets

6. **Advanced**
   - Database connection pool
   - Async ledger settings
   - Log levels
   - Debug mode

#### 2.7 Marketplace (Optional)

**When Enabled** (`TOKLIGENCE_MARKETPLACE_ENABLED=true`):

```
┌─────────────────────────────────────────┐
│ Tokligence Exchange Marketplace         │
├─────────────────────────────────────────┤
│ Browse Providers                        │
│                                         │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ GPT-4    │ │ Claude 3 │ │ Llama 3  │ │
│ │ OpenAI   │ │ Anthropic│ │ Meta     │ │
│ │ $0.03/1K │ │ $0.015/1K│ │ $0.001/1K│ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ Your Published Services                 │
│ ├─ My GPT Proxy      [Edit] [Unpublish]│
│ └─ [+ Publish new service]              │
└─────────────────────────────────────────┘
```

**Features**:
- Browse external providers
- Subscribe to third-party services
- Publish own adapters (if provider role enabled)
- Pricing and SLA visibility

---

## 3. Enterprise Edition UI Strategy

### Target Users
- Large enterprises (50+ users)
- Regulated industries
- Multi-tenant platforms
- Global organizations

### Key Characteristics
- **Governance & Compliance**: Full audit trails, data residency
- **Advanced RBAC**: Custom roles, teams, projects
- **Enterprise SSO**: SAML, OIDC, LDAP integration
- **SLA & Support**: 99.9% uptime, dedicated support

### UI Components

#### 3.1 Advanced Navigation

```
┌─────────────────────────────────────────┐
│ 🏢 Tokligence Gateway (Enterprise)      │
├─────────────────────────────────────────┤
│ Dashboard | Users | Teams | Projects    │
│ Analytics | Audit | Compliance | Settings│
│                                         │
│ Workspace: [Production ▼]              │
│   - Production (active)                 │
│   - Staging                             │
│   - Development                         │
│                                         │
│ [Profile: admin@enterprise.com ▼]      │
│   - My Profile                          │
│   - Security Settings                   │
│   - Admin Console                       │
│   - Sign Out                            │
└─────────────────────────────────────────┘
```

**Enterprise-Specific Navigation**:
- **Workspaces/Environments**: Separate prod/staging/dev
- **Teams**: Organizational units with hierarchies
- **Projects**: Cost centers and resource grouping
- **Compliance**: SOC2, GDPR, HIPAA dashboards
- **Audit Logs**: Comprehensive activity tracking

#### 3.2 Dashboard (Executive View)

```
┌─────────────────────────────────────────┐
│ Executive Dashboard                     │
│ Environment: [Production ▼]  Period: [Last 30 days ▼]│
│                                         │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ Total    │ │ Cost     │ │ Active   │ │
│ │ Spend    │ │ vs Budget│ │ Users    │ │
│ │ $12,450  │ │ 82%      │ │ 342/500  │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ Cost Trends (Monthly)                   │
│ [Advanced time-series chart with        │
│  budget line, forecast, and anomalies]  │
│                                         │
│ Department Breakdown                    │
│ ├─ Engineering      $5,200 (42%)       │
│ ├─ Product          $3,100 (25%)       │
│ ├─ Data Science     $2,800 (22%)       │
│ ├─ Marketing        $1,350 (11%)       │
│ └─ [Export detailed report]             │
│                                         │
│ Compliance Status                       │
│ ✓ SOC2 Type II      [View report]      │
│ ✓ GDPR Compliant    [Data map]         │
│ ⚠ HIPAA (1 pending) [Review findings]  │
└─────────────────────────────────────────┘
```

**Features**:
- **Multi-environment support**: Prod/Staging/Dev isolation
- **Budget tracking**: Alerts on overages
- **Forecasting**: ML-based cost predictions
- **Department/team attribution**: Chargeback reports
- **Compliance dashboards**: Real-time status

#### 3.3 User & Team Management

**Advanced RBAC**:

```
┌─────────────────────────────────────────┐
│ Users & Access Control                  │
├─────────────────────────────────────────┤
│ [Users] [Teams] [Roles] [SSO Config]   │
│                                         │
│ Users (342)                [+ Add User] │
│ Search: [____________]  Filters:        │
│ - Team: [All ▼]                         │
│ - Role: [All ▼]                         │
│ - Status: [Active ▼]                    │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ Name       Email        Team  Role │ │
│ ├────────────────────────────────────┤ │
│ │ Alice      alice@e.com  Eng   Dev  │ │
│ │ Bob        bob@e.com    Prod  Mgr  │ │
│ │ Charlie    charlie@e.c  Data  Sci  │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Bulk Actions:                           │
│ [Import from LDAP] [Export] [Assign Team]│
└─────────────────────────────────────────┘
```

**Teams View**:

```
┌─────────────────────────────────────────┐
│ Teams & Hierarchies         [+ New Team]│
├─────────────────────────────────────────┤
│ ├─ 🏢 Engineering (142 users)           │
│ │  ├─ Frontend (38)                     │
│ │  ├─ Backend (52)                      │
│ │  └─ DevOps (25)                       │
│ ├─ 🏢 Product (87 users)                │
│ ├─ 🏢 Data Science (54 users)           │
│ └─ 🏢 Marketing (42 users)              │
│                                         │
│ Team Detail: Engineering                │
│ - Owner: engineering-lead@e.com         │
│ - Budget: $15,000/month                 │
│ - Usage: $12,450 (83%)                  │
│ - API Keys: 28 active                   │
│ - Projects: 12                          │
│ [Edit Team] [View Members] [Analytics]  │
└─────────────────────────────────────────┘
```

**Custom Roles** (beyond gateway_user/gateway_admin):

```
┌─────────────────────────────────────────┐
│ Custom Roles                [+ New Role]│
├─────────────────────────────────────────┤
│ Role Name: API Viewer                   │
│ Description: Read-only access to APIs   │
│                                         │
│ Permissions:                            │
│ ☑ View dashboard                        │
│ ☑ View analytics                        │
│ ☐ Manage users                          │
│ ☐ Generate API keys                     │
│ ☑ View audit logs                       │
│ ☐ Modify settings                       │
│ ☑ View compliance reports               │
│                                         │
│ Assigned to: 23 users                   │
│ [Save Changes] [Delete Role]            │
└─────────────────────────────────────────┘
```

#### 3.4 Projects & Cost Centers

```
┌─────────────────────────────────────────┐
│ Projects & Cost Allocation  [+ New Project]│
├─────────────────────────────────────────┤
│ Project Name    Team        Budget  Usage│
├─────────────────────────────────────────┤
│ Mobile App      Engineering $5K    $4.2K │
│ Data Pipeline   Data Sci    $8K    $6.1K │
│ Marketing AI    Marketing   $2K    $1.8K │
│ Customer Bot    Product     $3K    $2.4K │
└─────────────────────────────────────────┘

Project Detail: Mobile App
├─ Owner: alice@e.com
├─ Team: Engineering > Backend
├─ Budget: $5,000/month (84% used)
├─ API Keys: 8 (with project tag)
├─ Models Used: gpt-4o (60%), claude-sonnet (40%)
└─ [View detailed analytics] [Export report]
```

**Features**:
- Tag API keys with project IDs
- Automatic cost attribution
- Budget alerts and approvals
- Project-level usage reports

#### 3.5 Advanced Analytics

**Multi-dimensional Analysis**:

1. **Query Builder** (drag-and-drop):
   - Dimensions: User, Team, Project, Model, Time, Region
   - Metrics: Requests, Tokens, Cost, Latency, Errors
   - Filters: Date range, status, provider
   - Visualizations: Line, bar, pie, heatmap, sankey

2. **Custom Reports**:
   - Saved queries for recurring reports
   - Scheduled email delivery
   - PDF/Excel export
   - Shared dashboards

3. **Anomaly Detection**:
   - ML-based outlier detection
   - Alerts on unusual patterns
   - Root cause analysis suggestions

4. **Benchmarking**:
   - Compare teams/projects
   - Industry benchmarks (if available)
   - Cost efficiency metrics

#### 3.6 Audit Logs & Compliance

**Comprehensive Audit Trail**:

```
┌─────────────────────────────────────────┐
│ Audit Logs                    [Export]  │
├─────────────────────────────────────────┤
│ Filters:                                │
│ - Time: [Last 7 days ▼]                │
│ - User: [All ▼]                         │
│ - Action: [All ▼]                       │
│ - Resource: [All ▼]                     │
│                                         │
│ ┌────────────────────────────────────┐ │
│ │ Time   User     Action    Resource │ │
│ ├────────────────────────────────────┤ │
│ │ 10:23  alice    Created   API Key  │ │
│ │ 10:15  bob      Modified  User     │ │
│ │ 09:58  charlie  Deleted   Project  │ │
│ └────────────────────────────────────┘ │
│                                         │
│ Detail View (expandable):               │
│ - IP Address: 192.168.1.42              │
│ - User Agent: Mozilla/5.0...            │
│ - Changed fields: role (user → admin)  │
│ - Approval: auto (within scope)         │
│ - Retention: 7 years (compliance)       │
└─────────────────────────────────────────┘
```

**Logged Events**:
- User login/logout
- API key creation/revocation
- Configuration changes
- Permission modifications
- Data access (read/write)
- Export operations
- Failed authentication attempts

**Compliance Dashboards**:

1. **SOC2**:
   - Access controls audit
   - Encryption status
   - Change management log
   - Incident response timeline

2. **GDPR**:
   - Data subject requests
   - Data retention policies
   - Cross-border transfer log
   - Right to be forgotten status

3. **HIPAA** (if applicable):
   - PHI access audit
   - Encryption verification
   - BAA status
   - Risk assessment

#### 3.7 Enterprise Settings

**Additional Sections**:

1. **SSO Configuration**:
   - SAML 2.0 setup
   - OIDC provider config
   - LDAP/Active Directory sync
   - Just-in-time provisioning
   - Attribute mapping

2. **Data Residency**:
   - Region selection (US, EU, APAC)
   - Data sovereignty compliance
   - Replication settings

3. **High Availability**:
   - Multi-region deployment
   - Failover configuration
   - Load balancing
   - Disaster recovery

4. **Integrations**:
   - Slack/Teams notifications
   - PagerDuty alerting
   - DataDog/Splunk APM
   - Jira ticketing
   - Webhook endpoints

5. **Custom Branding**:
   - Logo upload
   - Color scheme
   - Email templates
   - White-label domain

---

## 4. Progressive Disclosure: UX Patterns

### 4.1 Feature Gating Strategy

**How to Show/Hide Features Based on Edition**:

```typescript
// fe/src/config/features.ts
export const FEATURES = {
  PERSONAL: {
    auth: false,
    userManagement: false,
    apiKeyManagement: 'self-only',
    analytics: 'basic',
    marketplace: false,
    auditLogs: false,
    sso: false,
    teams: false,
    projects: false,
  },
  TEAM: {
    auth: true,
    userManagement: true,
    apiKeyManagement: 'admin',
    analytics: 'advanced',
    marketplace: 'optional',
    auditLogs: 'basic',
    sso: false,
    teams: false,
    projects: false,
  },
  ENTERPRISE: {
    auth: true,
    userManagement: true,
    apiKeyManagement: 'admin',
    analytics: 'enterprise',
    marketplace: 'full',
    auditLogs: 'comprehensive',
    sso: true,
    teams: true,
    projects: true,
  },
}

// Usage in components:
const features = useFeatures() // reads from backend config
if (features.userManagement) {
  // Show user management UI
}
```

### 4.2 Upgrade Prompts

**Strategic Placement**:

1. **Dashboard Widget** (Personal → Team):
   ```
   ┌────────────────────────────┐
   │ 💡 Upgrade to Team Edition │
   │                            │
   │ Get:                       │
   │ ✓ Multi-user access        │
   │ ✓ Usage analytics          │
   │ ✓ Centralized API keys     │
   │                            │
   │ [Learn More] [Dismiss]     │
   └────────────────────────────┘
   ```

2. **Feature Teaser** (Team → Enterprise):
   - Greyed-out menu items with lock icon
   - Hover tooltip: "Available in Enterprise"
   - Click → Upgrade modal with pricing

3. **Usage Threshold Triggers**:
   - >10 users: Suggest Team edition
   - >50 users: Suggest Enterprise edition
   - >$1000/month spend: Suggest cost optimization (Enterprise analytics)

### 4.3 Onboarding Flows

**Personal Edition** (2 steps):
1. Welcome screen → Quick start guide
2. Add API keys → Done

**Team Edition** (5 steps):
1. Welcome screen
2. Create admin account
3. Set up team
4. Invite users
5. Configure providers

**Enterprise Edition** (8 steps):
1. Welcome screen
2. Environment setup (prod/staging/dev)
3. SSO configuration
4. Team/department structure
5. Invite admins
6. Configure providers
7. Set budgets & alerts
8. Compliance setup

---

## 5. Visual Design System

### 5.1 Color Palette

**Base Colors** (Tailwind CSS):
- Primary: Slate (neutral, professional)
- Accent: Emerald (success, provider)
- Warning: Amber (alerts, warnings)
- Error: Rose (errors, critical)
- Info: Blue (consumer, info)

**Edition Branding**:
- Personal: Light theme, minimal colors
- Team: Balanced theme, team-focused colors
- Enterprise: Professional theme, corporate colors

### 5.2 Typography

- **Headings**: Inter or system font stack
- **Body**: System default for performance
- **Code**: JetBrains Mono or Fira Code

**Sizes**:
- H1: 2rem (dashboard titles)
- H2: 1.5rem (section headers)
- H3: 1.25rem (card titles)
- Body: 0.875rem (14px) - optimized for data-heavy UIs
- Small: 0.75rem (12px) - metadata, timestamps

### 5.3 Component Library

**Shared Components** (across all editions):
- Button variants: Primary, Secondary, Danger, Ghost
- Cards: Default, Outlined, Elevated
- Tables: Sortable, Filterable, Paginated
- Forms: Input, Select, Checkbox, Radio, Toggle
- Modals: Confirmation, Form, Detail view
- Toasts: Success, Error, Warning, Info
- Loading states: Skeleton, Spinner, Progress bar

**Edition-Specific Components**:
- Team: User table, Role selector, Team picker
- Enterprise: SSO config, Compliance dashboard, Audit viewer

---

## 6. Implementation Roadmap

### Phase 1: Personal Edition Refinement (Week 1-2)
- [ ] Simplify dashboard (remove team features)
- [ ] Create quick-start flow
- [ ] Add copy-to-clipboard for endpoint
- [ ] Hide authentication UI
- [ ] Test zero-config experience

### Phase 2: Team Edition Enhancement (Week 3-4)
- [ ] Build user management UI
- [ ] Implement API key admin panel
- [ ] Add team analytics dashboard
- [ ] Create usage breakdown reports
- [ ] Test role-based access control

### Phase 3: Enterprise Edition Foundation (Week 5-8)
- [ ] Design SSO integration UI
- [ ] Build team/project hierarchy
- [ ] Implement audit log viewer
- [ ] Create compliance dashboards
- [ ] Add custom role builder
- [ ] Test multi-environment support

### Phase 4: Polish & Optimization (Week 9-10)
- [ ] Responsive design testing
- [ ] Performance optimization
- [ ] Accessibility audit (WCAG 2.1)
- [ ] Browser compatibility testing
- [ ] Documentation and user guides

---

## 7. Technical Considerations

### 7.1 Backend API Extensions

**New Endpoints Needed**:

```
# Edition detection
GET /api/v1/edition
Response: { edition: "personal" | "team" | "enterprise" }

# Feature flags
GET /api/v1/features
Response: { auth: true, teams: false, ... }

# Team management (Enterprise)
GET    /api/v1/teams
POST   /api/v1/teams
GET    /api/v1/teams/{id}
PATCH  /api/v1/teams/{id}
DELETE /api/v1/teams/{id}

# Projects (Enterprise)
GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/{id}
PATCH  /api/v1/projects/{id}
DELETE /api/v1/projects/{id}

# Audit logs (Enterprise)
GET /api/v1/audit/logs
GET /api/v1/audit/export

# Analytics (Team & Enterprise)
GET /api/v1/analytics/query
POST /api/v1/analytics/custom-report

# SSO (Enterprise)
GET   /api/v1/sso/config
POST  /api/v1/sso/config
GET   /api/v1/sso/callback
```

### 7.2 Configuration Management

**Edition Detection**:

```go
// internal/config/config.go
type Edition string

const (
    EditionPersonal   Edition = "personal"
    EditionTeam       Edition = "team"
    EditionEnterprise Edition = "enterprise"
)

func (c *GatewayConfig) GetEdition() Edition {
    // Detect based on:
    // 1. License key validation (future)
    // 2. Environment variable: TOKLIGENCE_EDITION
    // 3. Docker image tag (personal/team/enterprise)
    // 4. Feature availability heuristic

    if c.AuthDisabled && !c.MarketplaceEnabled {
        return EditionPersonal
    }
    if c.SSOEnabled || c.TeamsEnabled {
        return EditionEnterprise
    }
    return EditionTeam
}
```

### 7.3 Frontend Architecture

**Context Provider for Edition**:

```typescript
// fe/src/context/EditionContext.tsx
export const EditionProvider = ({ children }) => {
  const { data: editionInfo } = useQuery({
    queryKey: ['edition'],
    queryFn: () => fetch('/api/v1/edition').then(r => r.json()),
  })

  return (
    <EditionContext.Provider value={editionInfo}>
      {children}
    </EditionContext.Provider>
  )
}

// Usage:
const { edition, features } = useEdition()
```

**Conditional Routing**:

```typescript
// fe/src/App.tsx
function App() {
  const { edition, features } = useEdition()

  return (
    <Routes>
      <Route path="/" element={<Dashboard />} />

      {features.userManagement && (
        <Route path="/users" element={<UsersPage />} />
      )}

      {features.teams && (
        <Route path="/teams" element={<TeamsPage />} />
      )}

      {features.auditLogs && (
        <Route path="/audit" element={<AuditPage />} />
      )}

      <Route path="/settings" element={<SettingsPage />} />
    </Routes>
  )
}
```

---

## 8. Success Metrics

### 8.1 Personal Edition Goals
- **Time to first request**: <5 minutes from install
- **Configuration steps**: ≤3 (add API keys only)
- **User satisfaction**: 4.5/5 stars for simplicity
- **Retention**: 60% weekly active after initial setup

### 8.2 Team Edition Goals
- **Team onboarding time**: <30 minutes
- **User management efficiency**: <2 minutes per user
- **Cost visibility**: 100% of usage attributed
- **Collaboration**: >80% teams use shared keys

### 8.3 Enterprise Edition Goals
- **SSO adoption**: >90% of users via SSO
- **Audit compliance**: 100% of actions logged
- **Cost allocation**: 100% usage tagged to projects
- **Uptime**: 99.9% SLA

---

## 9. Migration & Upgrade Paths

### 9.1 Personal → Team

**Data Migration**:
1. Export Personal edition config
2. Install Team edition
3. Import config via migration tool
4. Create admin account
5. Re-authenticate with session

**UI Changes**:
- Add login page
- Enable user management
- Show team dashboard

### 9.2 Team → Enterprise

**Data Migration**:
1. Export Team edition database (users, keys, ledger)
2. Install Enterprise edition
3. Import via enterprise migration tool
4. Configure SSO
5. Set up teams/projects

**UI Changes**:
- Add SSO login flow
- Enable teams/projects
- Show compliance dashboards

### 9.3 Backwards Compatibility

**API Versioning**:
- All new endpoints use `/api/v1/` prefix
- Existing endpoints remain stable
- Deprecated endpoints shown in docs with migration guide

---

## 10. Revenue Maximization: UI Design for Marketplace Growth

This section focuses on UI/UX strategies to maximize platform revenue through the marketplace.

### 10.1 Consumer Acquisition Funnel

**Goal**: Convert free users to paying marketplace consumers

```
Free User → Marketplace Browse → Subscribe → Regular Consumer → Power User
```

**UI Strategies**:

1. **Upgrade Prompts in Personal Edition**
   ```
   ┌─────────────────────────────────────────┐
   │ 💡 Save 30% with Marketplace Access     │
   ├─────────────────────────────────────────┤
   │ You're using OpenAI API directly at     │
   │ $0.030/1K tokens.                       │
   │                                         │
   │ Marketplace providers offer:            │
   │ • GPT-4o at $0.021/1K (30% cheaper)    │
   │ • No API key management                 │
   │ • Pay-as-you-go billing                 │
   │                                         │
   │ Potential savings: $127/month           │
   │                                         │
   │ [Upgrade to Team $29/mo] [Learn more]  │
   └─────────────────────────────────────────┘
   ```

2. **Cost Comparison Dashboard**
   - Show real-time savings comparison
   - "You could save $X by switching to marketplace provider Y"
   - Highlight when marketplace prices drop below direct API costs

3. **First-Purchase Incentives**
   - "Get $10 free credits on first marketplace subscription"
   - Show prominently after Team edition signup

### 10.2 Provider Acquisition Funnel

**Goal**: Convert consumers to revenue-generating providers

```
Consumer → Notice "Become Provider" CTA → Calculate Earnings → Publish Service → Active Provider
```

**UI Strategies**:

1. **Earnings Calculator Widget** (on all dashboards)
   ```
   ┌─────────────────────────────────────────┐
   │ 💰 You Could Be Earning                 │
   ├─────────────────────────────────────────┤
   │ Based on your consumption pattern:      │
   │                                         │
   │ You consumed: 500K tokens/month         │
   │ If you supplied at marketplace avg:     │
   │ • Gross revenue: ~$210/month            │
   │ • Platform fee (10%): -$21              │
   │ • Net earnings: ~$189/month             │
   │                                         │
   │ [Become a Provider] [Learn how]         │
   └─────────────────────────────────────────┘
   ```

2. **Provider Success Stories** (testimonials)
   ```
   ┌─────────────────────────────────────────┐
   │ 🌟 Provider Spotlight                   │
   ├─────────────────────────────────────────┤
   │ "I turned my idle GPU into $2,400/month │
   │  passive income with Tokligence."       │
   │                                         │
   │ - Alex Chen, GPU Farm Owner             │
   │                                         │
   │ [Start earning now →]                   │
   └─────────────────────────────────────────┘
   ```

3. **Quick Setup Wizard** (minimize friction)
   - One-click provider registration
   - Auto-detect existing API keys
   - Suggest competitive pricing
   - "Launch in 5 minutes" messaging

### 10.3 Transaction Volume Optimization

**Goal**: Increase tokens traded through marketplace

**UI Strategies**:

1. **Default to Marketplace Routes**
   - When both marketplace and direct API available, default to marketplace
   - Show checkbox: "Always use marketplace when available"
   - Subtle nudge: "Using marketplace helps support the platform"

2. **Usage Leaderboards** (gamification)
   ```
   ┌─────────────────────────────────────────┐
   │ 🏆 Top Providers This Month             │
   ├─────────────────────────────────────────┤
   │ 1. 🥇 MegaGPU Farm   $12,450   2.1M tok │
   │ 2. 🥈 FastLLM Pro    $8,230    1.4M tok │
   │ 3. 🥉 CloudAI Hub    $6,100    980K tok │
   │                                         │
   │ Your rank: #47 ($1,278)                 │
   │ [See full leaderboard]                  │
   └─────────────────────────────────────────┘
   ```

3. **Volume Discounts for Consumers**
   - Tiered pricing display: "Unlock 5% off at 1M tokens/month"
   - Progress bar showing next discount tier
   - Encourages higher spending

4. **Provider Incentives for Volume**
   - "Reduced platform fee for 1M+ tokens/month (10% → 8%)"
   - "Featured placement for top 10 providers"
   - Show potential earnings increase

### 10.4 Pricing Optimization UI

**Goal**: Help providers price competitively while maximizing platform revenue

**Provider Pricing Dashboard**:
```
┌─────────────────────────────────────────┐
│ 📊 Pricing Strategy Optimizer           │
├─────────────────────────────────────────┤
│ Current Price: $0.0220/1K               │
│                                         │
│ Market Analysis:                        │
│ • Competitor avg: $0.0210               │
│ • Your position: +4.8% premium          │
│ • Suggested: $0.0200 (-9% price)        │
│                                         │
│ Impact Forecast:                        │
│ If you lower to $0.0200:                │
│ • Expected demand: +35%                 │
│ • Monthly revenue: $1,204 (+12%)        │
│ • Customer gain: ~48 new subscribers    │
│                                         │
│ [Apply Suggested Price] [Custom Price]  │
└─────────────────────────────────────────┘
```

**Features**:
- AI-powered pricing recommendations
- Elasticity simulator (price vs volume)
- Competitor benchmarking
- Real-time demand forecast

### 10.5 Retention Mechanisms

**Goal**: Keep users subscribed and transacting

**Consumer Retention**:

1. **Subscription Management Dashboard**
   ```
   ┌─────────────────────────────────────────┐
   │ Your Active Subscriptions (5)           │
   ├─────────────────────────────────────────┤
   │ GPT-4o Budget                           │
   │ Last used: 2 hours ago                  │
   │ Savings vs official: $18/month          │
   │ [Manage] [Cancel]                       │
   │                                         │
   │ Claude Sonnet Pro                       │
   │ ⚠️ Unused for 14 days                   │
   │ Suggestion: Pause to save $12/month     │
   │ [Pause] [Keep active]                   │
   └─────────────────────────────────────────┘
   ```

2. **Churn Prevention**
   - Exit survey when canceling: "What made you leave?"
   - Offer discount/credits before finalizing cancellation
   - Suggest cheaper alternatives

**Provider Retention**:

1. **Performance Alerts**
   ```
   ┌─────────────────────────────────────────┐
   │ ⚠️ Action Required                      │
   ├─────────────────────────────────────────┤
   │ Your service "My GPT-4o" has:           │
   │ • 3 customer complaints (4.2★ → 3.8★)   │
   │ • 12% higher latency than competitors   │
   │                                         │
   │ Risk: Potential delisting               │
   │                                         │
   │ Recommendations:                        │
   │ • Scale up capacity                     │
   │ • Lower price by 5%                     │
   │ • Add free trial tokens                 │
   │                                         │
   │ [Improve performance] [Contact support] │
   └─────────────────────────────────────────┘
   ```

2. **Growth Coaching**
   - Monthly provider performance report
   - Personalized growth tips
   - "Providers like you earn X% more by doing Y"

### 10.6 Marketplace Network Effects

**Goal**: Make platform more valuable as more users join

**UI Features**:

1. **Referral Program**
   ```
   ┌─────────────────────────────────────────┐
   │ 🎁 Invite & Earn                        │
   ├─────────────────────────────────────────┤
   │ Invite friends to Tokligence:           │
   │ • They get $20 free credits             │
   │ • You get $10 per signup                │
   │ • Plus 5% of their first-year spending  │
   │                                         │
   │ Your referrals: 8 signups, $142 earned  │
   │                                         │
   │ Your referral link:                     │
   │ tokligence.ai/r/abc123xyz               │
   │ [Copy link] [Share via email]           │
   └─────────────────────────────────────────┘
   ```

2. **Community Features**
   - Provider directory with profiles
   - Customer reviews and ratings (with moderation)
   - Provider response to reviews
   - "Verified" badges for trusted providers

3. **Marketplace Stats** (build trust)
   ```
   ┌─────────────────────────────────────────┐
   │ 📈 Marketplace Statistics               │
   ├─────────────────────────────────────────┤
   │ • 12,450 active consumers               │
   │ • 342 verified providers                │
   │ • $2.4M tokens traded this month        │
   │ • 99.2% uptime (last 30 days)           │
   │ • Avg savings: 22% vs direct APIs       │
   └─────────────────────────────────────────┘
   ```

### 10.7 Revenue Analytics (Platform Admin)

**Internal Dashboard for Tokligence Team**:

```
┌─────────────────────────────────────────┐
│ 💼 Platform Revenue Dashboard           │
├─────────────────────────────────────────┤
│ This Month (Nov 2025)                   │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│ │ GMV      │ │ Take Rate│ │ Platform │ │
│ │ (Gross)  │ │ Revenue  │ │ Revenue  │ │
│ │ $142.5K  │ │ $14.25K  │ │ $38.7K   │ │
│ └──────────┘ └──────────┘ └──────────┘ │
│                                         │
│ Revenue Breakdown:                      │
│ • Transaction fees (10%): $14,250       │
│ • Team subscriptions: $18,420           │
│ • Enterprise contracts: $6,000          │
│                                         │
│ Unit Economics:                         │
│ • Avg consumer LTV: $847                │
│ • Avg provider LTV: $3,240              │
│ • CAC (consumer): $23                   │
│ • CAC (provider): $67                   │
│ • Payback period: 2.1 months            │
│                                         │
│ Growth Metrics:                         │
│ • MoM GMV growth: +18%                  │
│ • New consumers: 342 (+12%)             │
│ • New providers: 28 (+24%)              │
│ • Churn rate: 3.2% (consumers)          │
│ • Churn rate: 1.8% (providers)          │
└─────────────────────────────────────────┘
```

### 10.8 A/B Testing Framework

**Test hypotheses to optimize conversion**:

| Experiment | Hypothesis | Metric |
|------------|-----------|--------|
| Provider CTA placement | Top-right vs bottom banner | Provider signup rate |
| Pricing display | Per 1K vs per token | Subscription conversion |
| Free trial amount | 10K vs 50K tokens | Trial → paid conversion |
| Platform fee visibility | Hidden vs transparent | Provider trust score |
| Referral bonus amount | $5 vs $10 vs $20 | Referral participation |

**UI for A/B Testing** (admin tool):
- Visual experiment builder
- Real-time results dashboard
- Statistical significance calculator
- Automatic winner deployment

### 10.9 Key Revenue Metrics to Display

**For Platform Health**:
- Gross Marketplace Volume (GMV)
- Take rate (platform fee %)
- Net revenue (fees + subscriptions)
- Monthly Recurring Revenue (MRR)
- Annual Run Rate (ARR)

**For User Engagement**:
- Daily Active Users (DAU)
- Monthly Active Users (MAU)
- Tokens traded per user
- Subscription renewal rate
- Net Promoter Score (NPS)

**For Provider Ecosystem**:
- Number of active providers
- Avg revenue per provider
- Provider retention rate
- Service quality score (avg rating)
- Time to first sale (new providers)

---

## 11. Open Questions & Future Considerations

### 11.1 Open Questions

1. **Licensing Model**:
   - License key validation mechanism?
   - Online vs offline activation?
   - Trial periods for Team/Enterprise?

2. **Data Privacy**:
   - Where to store usage analytics (local vs cloud)?
   - Opt-in telemetry for Personal edition?
   - GDPR/CCPA compliance for user data?

3. **Customization**:
   - Allow custom themes per team?
   - White-label for Enterprise customers?
   - Plugin system for custom dashboards?

4. **Mobile Support**:
   - Responsive web app sufficient?
   - Native mobile app needed?
   - Progressive Web App (PWA)?

### 10.2 Future Features

**Personal Edition**:
- [ ] Browser extension for quick endpoint switching
- [ ] Desktop app (Electron) for offline config
- [ ] Model playground for testing prompts

**Team Edition**:
- [ ] Slack/Teams integration for usage alerts
- [ ] Shared prompt library
- [ ] Team-wide model fine-tuning management

**Enterprise Edition**:
- [ ] Multi-region deployment UI
- [ ] Custom SLA management
- [ ] Advanced threat detection (anomaly in API usage)
- [ ] Integration marketplace (Zapier, etc.)

---

## Appendix A: Wireframe References

### A.1 Personal Dashboard (Desktop)
See: `docs/wireframes/personal-dashboard-desktop.png` (to be created)

### A.2 Team User Management (Desktop)
See: `docs/wireframes/team-users-desktop.png` (to be created)

### A.3 Enterprise Analytics (Desktop)
See: `docs/wireframes/enterprise-analytics-desktop.png` (to be created)

### A.4 Responsive Views (Mobile/Tablet)
See: `docs/wireframes/responsive-layouts.png` (to be created)

---

## Appendix B: Competitor Analysis

| Competitor | Personal | Team | Enterprise | Notes |
|------------|----------|------|------------|-------|
| OpenAI API Platform | N/A | ✓ | ✓ | Organization-based, simple UI |
| Anthropic Console | N/A | ✓ | ✓ | Minimal team features |
| Azure OpenAI | N/A | N/A | ✓ | Enterprise-only, complex |
| Portkey.ai | ✓ | ✓ | ✓ | Strong analytics, lacks simplicity |
| Helicone | ✓ | ✓ | ✓ | Good observability, basic RBAC |

**Tokligence Advantage**:
- Only gateway with true Personal edition (zero-config)
- Best-in-class protocol translation (Codex ↔ Anthropic)
- Unified interface for all providers
- **Dual-sided marketplace**: Buy AND sell tokens (unique vs competitors)
- **Provider monetization**: Turn idle GPUs into revenue streams
- **Transparent pricing**: Real-time cost comparison across providers

---

## Appendix C: Glossary

**Edition Types**:
- **Personal Edition**: Single-user, marketplace consumer access (browse/buy tokens), provider features locked
- **Team Edition**: Multi-user, RBAC, full marketplace (consumer + provider), team collaboration
- **Enterprise Edition**: SSO, teams, projects, compliance, full marketplace + white-label

**Marketplace Terms**:
- **Consumer**: User who buys/consumes tokens from marketplace providers
- **Provider**: User who sells/supplies tokens via marketplace (earns revenue)
- **GMV** (Gross Marketplace Volume): Total value of all transactions
- **Take Rate**: Platform fee percentage (e.g., 10% of provider revenue)
- **Settlement**: Payment processing between consumers, providers, and platform
- **Supply**: Token direction when provider sells (opposite of consume)
- **Consume**: Token direction when consumer buys (opposite of supply)

**Technical Terms**:
- **Progressive Disclosure**: Showing complexity only when needed
- **RBAC**: Role-Based Access Control
- **SSO**: Single Sign-On (SAML, OIDC)
- **Audit Log**: Immutable record of all user actions
- **Cost Attribution**: Linking usage to users/teams/projects
- **Dual-Sided Marketplace**: Platform with both buyers (consumers) and sellers (providers)

---

## Document Change Log

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| v1.0 | 2025-11-22 | Product Planning | Initial draft |
| v2.0 | 2025-11-22 | Product Planning | **Major revision**: Added dual-sided marketplace focus, consumer/provider UX patterns, revenue maximization strategies, settlement UI, and marketplace-specific features |
| v2.1 | 2025-11-22 | Product Planning | **Critical update**: Marketplace enabled for ALL editions by default (Personal = consumer only, Team/Enterprise = full). Updated Personal edition UI with marketplace discovery and provider upgrade prompts. |

---

**End of Document**
