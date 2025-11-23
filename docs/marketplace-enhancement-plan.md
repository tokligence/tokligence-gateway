# Marketplace Enhancement Plan

## Current Issues & Missing Features

### 1. Admin Users Page Issues
**Problem**: `/admin/users` shows "Failed to load users"
- Backend endpoint may not exist or is not properly configured
- Need to verify API endpoint exists in backend
- Need proper authentication/authorization for admin endpoints

### 2. Service Details Modal
**Current State**: "Details" button on marketplace services has no functionality
**Required Features** (inspired by OpenRouter):

#### Service Detail Modal Should Include:
- **Basic Information**
  - Service name and description
  - Model family (GPT-4, Claude, Llama, etc.)
  - Provider information (name, verification status)
  - Price per 1K tokens (prompt + completion separately if different)

- **Technical Specifications**
  - Context window (e.g., 8K, 32K, 128K tokens)
  - Max output tokens
  - Supported features (function calling, vision, streaming, etc.)
  - API compatibility (OpenAI, Anthropic, etc.)

- **Provider Quality Metrics**
  - **Geographic Information**
    - Provider country/region
    - Data center location(s)
    - Compliance certifications (GDPR, SOC2, etc.)

  - **Performance Metrics**
    - Average latency (p50, p95, p99)
    - Uptime percentage (7-day, 30-day)
    - Throughput capacity (requests/sec, tokens/sec)
    - Queue depth/wait time

  - **Availability**
    - Available hours (24/7, business hours, timezone)
    - Scheduled maintenance windows
    - Historical availability data

  - **Reviews & Ratings**
    - Overall rating (1-5 stars)
    - Number of reviews
    - Recent reviews with feedback
    - Usage statistics (total tokens served, active users)

- **Trial Information**
  - Free trial tokens available
  - Trial limitations/restrictions
  - How to activate trial

- **Actions**
  - "Start Using" button (subscribe/enable service)
  - "Test API" button (opens API testing interface)
  - "View Documentation" link
  - "Report Issue" link

### 3. "Start Using" Workflow
**Current State**: Button exists but no functionality
**Required Workflow**:

#### For Consumers:
1. Click "Start Using" → Open subscription modal
2. Show service details recap:
   - Price breakdown
   - Expected monthly cost calculator (based on estimated usage)
   - Trial offer if available
3. Confirm subscription
4. Generate/display API credentials:
   - Endpoint URL
   - API key (show once with copy button)
   - Code examples (curl, Python, JavaScript, etc.)
5. Show "Get Started" guide:
   - Quick start code snippets
   - Link to full documentation
   - Link to usage dashboard

#### For Providers (Publishing Services):
1. Navigate to Provider Dashboard
2. Click "Publish New Service"
3. Multi-step form:
   - **Step 1: Basic Info**
     - Service name
     - Description
     - Model family selection
     - Base model (if applicable)

   - **Step 2: Pricing**
     - Price per 1K prompt tokens
     - Price per 1K completion tokens
     - Optional: Bulk pricing tiers
     - Trial tokens offer (optional)

   - **Step 3: Technical Details**
     - API endpoint URL
     - Authentication method
     - Context window size
     - Max output tokens
     - Supported features checkboxes

   - **Step 4: Infrastructure Info**
     - Geographic location (country, region, city)
     - Data center details
     - Compliance certifications
     - Availability schedule

   - **Step 5: Performance Commitments**
     - Expected latency targets (p50, p95, p99)
     - Uptime SLA (%)
     - Throughput capacity
     - Rate limits

   - **Step 6: Review & Publish**
     - Preview service card
     - Terms acceptance
     - Submit for review (if marketplace requires approval)
     - Or publish immediately (if auto-approve enabled)

### 4. Model Selection & Filtering (OpenRouter-style)

#### Enhanced Filter Options:
- **By Model Family**
  - GPT (OpenAI)
  - Claude (Anthropic)
  - Llama (Meta)
  - Gemini (Google)
  - Mistral
  - Custom/Other

- **By Context Window**
  - < 8K tokens
  - 8K - 32K
  - 32K - 128K
  - > 128K tokens

- **By Features**
  - Function calling
  - Vision/multimodal
  - Streaming
  - JSON mode
  - Custom fine-tuned

- **By Price Range**
  - Free tier available
  - < $0.01 per 1K tokens
  - $0.01 - $0.10 per 1K tokens
  - > $0.10 per 1K tokens

- **By Provider Location**
  - Geographic region selector
  - Latency from user location estimate

- **By Performance**
  - Uptime > 99%
  - Uptime > 99.9%
  - Low latency (< 100ms p95)
  - High throughput

- **By Availability**
  - 24/7 availability
  - Business hours only
  - Currently available

#### Search & Sort:
- **Search**: Text search across service name, description, model family
- **Sort Options**:
  - Price (low to high, high to low)
  - Rating (best first)
  - Popularity (most used)
  - Newest
  - Latency (fastest first)
  - Uptime (highest first)

### 5. Provider Dashboard Enhancements

#### Real Revenue & Usage Data:
Current state shows mock data. Need to integrate:
- Actual tokens supplied (from ledger)
- Real revenue calculations
- Platform fee deductions
- Payout schedule and amounts

#### Service Performance Monitoring:
- Per-service metrics dashboard:
  - Request volume (chart)
  - Token usage (chart)
  - Revenue per service (chart)
  - Error rates
  - Average latency
  - Customer satisfaction ratings

#### Customer Analytics:
- Top customers table with:
  - Real usage data
  - Revenue contribution
  - Usage trends
  - Customer retention metrics

### 6. Backend API Requirements

#### New Endpoints Needed:

**Services API** (Gateway):
```
GET /v1/services?scope={all|subscribed|owned}
  - Returns: { services: Service[] }

GET /v1/services/:id
  - Returns: { service: ServiceDetail }

POST /v1/services/:id/subscribe
  - Creates subscription, returns API credentials

DELETE /v1/services/:id/subscribe
  - Unsubscribes from service
```

**Providers API** (Gateway):
```
GET /v1/providers
  - Returns: { providers: Provider[] }

GET /v1/providers/:id
  - Returns: { provider: ProviderDetail, services: Service[] }
```

**Provider Management API** (Gateway):
```
POST /v1/provider/services
  - Publish new service
  - Body: ServiceCreatePayload

PUT /v1/provider/services/:id
  - Update service details

DELETE /v1/provider/services/:id
  - Unpublish service

GET /v1/provider/analytics
  - Returns provider analytics data
```

**Marketplace API** (NEW - Separate Service):
```
GET /marketplace/services
  - Public marketplace browse
  - Supports filtering, sorting, search

GET /marketplace/services/:id/metrics
  - Service performance metrics (public)
  - Uptime, latency, reviews, ratings

POST /marketplace/services/:id/reviews
  - Submit service review

GET /marketplace/providers/:id
  - Provider profile and services
```

### 7. Service Schema Enhancement

```typescript
interface Service {
  id: string
  name: string
  description: string
  modelFamily: 'gpt' | 'claude' | 'llama' | 'gemini' | 'mistral' | 'other'
  baseModel?: string

  // Pricing
  pricePer1KTokens: number
  pricingTiers?: { tokens: number; price: number }[]
  trialTokens?: number

  // Provider
  providerId: string
  providerName: string
  providerVerified: boolean

  // Technical specs
  contextWindow: number
  maxOutputTokens: number
  features: {
    functionCalling: boolean
    vision: boolean
    streaming: boolean
    jsonMode: boolean
  }
  apiCompatibility: string[] // ['openai', 'anthropic']

  // Infrastructure
  geographic: {
    country: string
    region: string
    city?: string
    dataCenters: string[]
  }
  compliance: string[] // ['GDPR', 'SOC2', 'HIPAA']

  // Performance
  metrics: {
    uptime7d: number
    uptime30d: number
    latencyP50: number
    latencyP95: number
    latencyP99: number
    throughputRps: number
    throughputTps: number
  }

  // Availability
  availability: {
    schedule: '24/7' | 'business_hours' | 'custom'
    timezone?: string
    customHours?: { start: string; end: string }[]
    maintenanceWindows?: { start: string; end: string }[]
  }

  // Social proof
  rating: number // 1-5
  reviewCount: number
  usageStats: {
    totalTokensServed: number
    activeUsers: number
    monthlyRequests: number
  }

  // Metadata
  status: 'active' | 'maintenance' | 'deprecated'
  createdAt: string
  updatedAt: string
}
```

## Implementation Priority

### Phase 1: Critical Functionality (Week 1)
1. Fix admin users API endpoint
2. Implement service details modal with basic info
3. Implement "Start Using" workflow with API credential generation
4. Add service subscription state management

### Phase 2: Provider Features (Week 2)
5. Implement "Publish New Service" multi-step form
6. Add service schema with all fields
7. Integrate real usage & revenue data in Provider Dashboard
8. Add per-service analytics

### Phase 3: Marketplace Enhancements (Week 3)
9. Implement advanced filtering (OpenRouter-style)
10. Add search functionality
11. Add geographic/latency filtering
12. Implement performance metrics display

### Phase 4: Quality & Social Features (Week 4)
13. Add reviews and ratings system
14. Implement service monitoring dashboard
15. Add uptime/latency tracking
16. Customer analytics for providers

## Open Questions

1. **Marketplace Approval Process**: Should new services require admin approval before going live?
2. **Payment Integration**: How do providers get paid? Stripe Connect? Manual payouts?
3. **SLA Enforcement**: How to handle providers who don't meet their SLA commitments?
4. **Geographic Data**: Do we integrate with IP geolocation to show latency estimates?
5. **Monitoring**: Do we run our own uptime monitors or rely on provider self-reporting?
6. **API Testing**: Should we provide an in-browser API testing tool like Postman?

## Next Steps

1. Review this plan with team
2. Prioritize features based on business needs
3. Design database schema for service metadata
4. Create API specification document
5. Break down into implementable tasks
6. Start with Phase 1 critical functionality
