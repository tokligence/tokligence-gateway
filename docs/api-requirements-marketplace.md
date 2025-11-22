# Marketplace API Requirements

**Version**: v1.0
**Date**: 2025-11-22
**Status**: Specification for Implementation

---

## Overview

This document specifies the backend API endpoints required to support the marketplace UI implementation. The frontend has been built with placeholders for these APIs, which need to be implemented on the backend.

---

## 1. Edition & Feature Detection

### GET /api/v1/edition

**Purpose**: Get current gateway edition and available features

**Response**:
```json
{
  "edition": "personal" | "team" | "enterprise",
  "features": {
    "marketplaceConsumer": true,
    "marketplaceProvider": true,
    "multiUser": false,
    "userRoles": false,
    "adminPanel": false,
    "authRequired": false,
    "sso": false,
    "teams": false,
    "projects": false,
    "auditLogs": false,
    "advancedAnalytics": false,
    "customBranding": false,
    "apiKeyManagement": "self" | "admin" | "scoped"
  }
}
```

**Authentication**: Optional (can be called without auth)

**Notes**:
- For now, edition detection can be based on:
  - Environment variable: `TOKLIGENCE_EDITION`
  - License key validation (future)
  - Auth enabled = team/enterprise; auth disabled = personal
- All editions support both buying and selling tokens

---

## 2. Service Activation (Consumer Actions - Pay-as-you-go)

### POST /api/v1/services/:serviceId/activate

**Purpose**: Activate a marketplace service for pay-as-you-go usage

**Request Body**:
```json
{
  "payment_method_id": "pm_1234567890",  // optional, use default if not provided
  "accept_trial": true  // whether to accept free trial tokens
}
```

**Response**:
```json
{
  "activation": {
    "id": 123,
    "service_id": 456,
    "user_id": 789,
    "status": "active",
    "trial_tokens_remaining": 10000,
    "activated_at": "2025-11-22T10:30:00Z"
  },
  "service": {
    "id": 456,
    "name": "GPT-4o Budget",
    "model_family": "gpt-4o",
    "price_per_1k_tokens": 0.021,
    "provider_id": 12
  }
}
```

**Errors**:
- `400 Bad Request`: Service not found or not available
- `402 Payment Required`: Payment method required and not provided
- `409 Conflict`: Already activated this service

**Notes**:
- Pay-as-you-go model: users are billed based on actual token consumption
- No monthly fees or subscription charges
- Billing occurs at end of billing period (e.g., monthly) based on usage
- Trial tokens are consumed first before charging

---

### GET /api/v1/services/active

**Purpose**: List user's active services (pay-as-you-go)

**Query Parameters**:
- `status`: `active` | `paused` | `canceled` (default: `active`)

**Response**:
```json
{
  "activations": [
    {
      "id": 123,
      "service": {
        "id": 456,
        "name": "GPT-4o Budget",
        "model_family": "gpt-4o",
        "price_per_1k_tokens": 0.021,
        "provider_id": 12
      },
      "status": "active",
      "trial_tokens_remaining": 5000,
      "tokens_consumed_this_month": 45000,
      "cost_this_month": 0.945,
      "activated_at": "2025-11-22T10:30:00Z",
      "next_billing_date": "2025-12-01T00:00:00Z"
    }
  ]
}
```

---

### PATCH /api/v1/services/:serviceId/activation

**Purpose**: Update service activation status (pause/resume/deactivate)

**Request Body**:
```json
{
  "status": "paused" | "active" | "deactivated"
}
```

**Response**:
```json
{
  "activation": {
    "id": 123,
    "service_id": 456,
    "status": "paused",
    "paused_at": "2025-11-22T15:00:00Z"
  }
}
```

---

### DELETE /api/v1/subscriptions/:id

**Purpose**: Cancel subscription (alias for PATCH with status=canceled)

**Response**:
```json
{
  "message": "Subscription canceled successfully",
  "final_bill": {
    "amount": 12.45,
    "billing_date": "2025-12-01T00:00:00Z"
  }
}
```

---

## 3. Provider Service Publishing

### POST /api/v1/provider/services

**Purpose**: Publish a new token service to the marketplace

**Request Body**:
```json
{
  "name": "My GPT-4o Proxy",
  "model_family": "gpt-4o",
  "description": "High-performance GPT-4o with 99.9% uptime",
  "price_per_1k_tokens": 0.022,
  "trial_tokens": 10000,
  "max_requests_per_minute": 1000,
  "max_tokens_per_day": 10000000,
  "upstream_config": {
    "provider": "openai",
    "api_key_ref": "my_openai_key",  // reference to stored API key
    "base_url": "https://api.openai.com/v1"  // optional
  },
  "auto_pause_on_failure": true,
  "auto_pause_on_quota_exceeded": true
}
```

**Response**:
```json
{
  "service": {
    "id": 789,
    "provider_id": 12,
    "name": "My GPT-4o Proxy",
    "model_family": "gpt-4o",
    "price_per_1k_tokens": 0.022,
    "trial_tokens": 10000,
    "status": "active",
    "created_at": "2025-11-22T16:00:00Z",
    "marketplace_url": "https://marketplace.tokligence.ai/services/789"
  }
}
```

**Errors**:
- `400 Bad Request`: Invalid pricing or configuration
- `403 Forbidden`: User is not registered as a provider
- `409 Conflict`: Service with same name already exists

---

### GET /api/v1/provider/services

**Purpose**: List provider's published services

**Query Parameters**:
- `status`: `active` | `paused` | `draft` (default: `active`)

**Response**:
```json
{
  "services": [
    {
      "id": 789,
      "name": "My GPT-4o Proxy",
      "model_family": "gpt-4o",
      "price_per_1k_tokens": 0.022,
      "status": "active",
      "subscribers_count": 134,
      "tokens_supplied_this_month": 1800000,
      "revenue_this_month": 890.00,
      "uptime_percentage": 99.5,
      "avg_rating": 4.6,
      "review_count": 89,
      "created_at": "2025-11-22T16:00:00Z"
    }
  ]
}
```

---

### PATCH /api/v1/provider/services/:id

**Purpose**: Update service configuration or pricing

**Request Body**:
```json
{
  "price_per_1k_tokens": 0.020,  // optional
  "trial_tokens": 15000,  // optional
  "status": "active" | "paused",  // optional
  "max_requests_per_minute": 2000  // optional
}
```

**Response**:
```json
{
  "service": {
    "id": 789,
    "price_per_1k_tokens": 0.020,
    "updated_at": "2025-11-22T17:00:00Z"
  },
  "pricing_change": {
    "effective_date": "2025-12-01T00:00:00Z",
    "notice_sent_to_subscribers": true
  }
}
```

**Notes**:
- Price changes should have a grace period (e.g., 7 days) before taking effect
- Subscribers should be notified of price changes

---

### DELETE /api/v1/provider/services/:id

**Purpose**: Delete/delist service from marketplace

**Response**:
```json
{
  "message": "Service delisted successfully",
  "active_subscribers": 134,
  "notice": "Existing subscribers will be migrated within 30 days"
}
```

---

## 4. Provider Revenue & Analytics

### GET /api/v1/provider/revenue

**Purpose**: Get provider revenue summary

**Query Parameters**:
- `period`: `current_month` | `last_month` | `last_30_days` | `all_time` (default: `current_month`)

**Response**:
```json
{
  "period": "current_month",
  "period_start": "2025-11-01T00:00:00Z",
  "period_end": "2025-11-30T23:59:59Z",
  "summary": {
    "tokens_supplied": 2800000,
    "gross_revenue": 1420.00,
    "platform_fee": 142.00,
    "platform_fee_percentage": 10,
    "net_revenue": 1278.00,
    "active_services": 2,
    "active_subscribers": 207
  },
  "by_service": [
    {
      "service_id": 789,
      "service_name": "My GPT-4o Proxy",
      "tokens_supplied": 1800000,
      "gross_revenue": 890.00,
      "subscriber_count": 134
    },
    {
      "service_id": 790,
      "service_name": "My Claude Sonnet API",
      "tokens_supplied": 1000000,
      "gross_revenue": 530.00,
      "subscriber_count": 73
    }
  ],
  "next_payout": {
    "amount": 1278.00,
    "scheduled_date": "2025-12-01T00:00:00Z",
    "payout_method": "bank_account_****6789"
  }
}
```

---

### GET /api/v1/provider/revenue/trends

**Purpose**: Get daily revenue trends for charting

**Query Parameters**:
- `days`: number of days (default: 30, max: 365)

**Response**:
```json
{
  "trends": [
    {
      "date": "2025-11-01",
      "tokens_supplied": 95000,
      "revenue": 47.50,
      "unique_consumers": 42
    },
    {
      "date": "2025-11-02",
      "tokens_supplied": 102000,
      "revenue": 51.00,
      "unique_consumers": 45
    }
    // ... more days
  ]
}
```

---

### GET /api/v1/provider/customers

**Purpose**: Get top customers by volume

**Query Parameters**:
- `limit`: number (default: 10, max: 100)
- `period`: `current_month` | `last_30_days` | `all_time` (default: `current_month`)

**Response**:
```json
{
  "customers": [
    {
      "customer_id": "customer_abc",  // anonymized ID
      "tokens_consumed": 45000,
      "revenue_generated": 142.00,
      "service_id": 789,
      "service_name": "My GPT-4o Proxy",
      "first_transaction": "2025-11-05T10:00:00Z",
      "last_transaction": "2025-11-22T09:30:00Z"
    },
    {
      "customer_id": "customer_xyz",
      "tokens_consumed": 32000,
      "revenue_generated": 98.00,
      "service_id": 789,
      "service_name": "My GPT-4o Proxy",
      "first_transaction": "2025-11-10T14:00:00Z",
      "last_transaction": "2025-11-22T11:00:00Z"
    }
  ]
}
```

**Privacy Notes**:
- Customer IDs should be anonymized/hashed
- Do not expose customer email or personal information
- Only show aggregated consumption data

---

## 5. Payout Management

### GET /api/v1/provider/payout-method

**Purpose**: Get current payout method configuration

**Response**:
```json
{
  "payout_method": {
    "type": "bank_account",
    "bank_name": "Chase Bank",
    "account_last_4": "6789",
    "routing_number_last_4": "0123",
    "verified": true,
    "added_at": "2025-10-01T12:00:00Z"
  }
}
```

---

### POST /api/v1/provider/payout-method

**Purpose**: Add or update payout method

**Request Body**:
```json
{
  "type": "bank_account",
  "country": "US",
  "currency": "USD",
  "account_number": "123456789",
  "routing_number": "110000000",
  "account_holder_name": "John Doe"
}
```

**Response**:
```json
{
  "payout_method": {
    "type": "bank_account",
    "account_last_4": "6789",
    "verification_required": true,
    "verification_method": "microdeposits"
  },
  "message": "Payout method added. Please verify within 3-5 business days."
}
```

---

### GET /api/v1/provider/payouts

**Purpose**: Get payout history

**Query Parameters**:
- `limit`: number (default: 10, max: 100)
- `status`: `pending` | `paid` | `failed` (optional)

**Response**:
```json
{
  "payouts": [
    {
      "id": 1001,
      "amount": 1278.00,
      "status": "paid",
      "scheduled_date": "2025-11-01T00:00:00Z",
      "paid_date": "2025-11-01T08:30:00Z",
      "payout_method": "bank_account_****6789",
      "transaction_id": "po_1234567890"
    },
    {
      "id": 1002,
      "amount": 1142.30,
      "status": "paid",
      "scheduled_date": "2025-10-01T00:00:00Z",
      "paid_date": "2025-10-01T09:15:00Z",
      "payout_method": "bank_account_****6789",
      "transaction_id": "po_0987654321"
    }
  ]
}
```

---

## 6. Marketplace Statistics & Reviews

### GET /api/v1/marketplace/stats

**Purpose**: Get global marketplace statistics (for trust building)

**Response**:
```json
{
  "stats": {
    "active_consumers": 12450,
    "verified_providers": 342,
    "total_tokens_traded_this_month": 2400000000,
    "total_services": 1234,
    "marketplace_uptime_percentage": 99.2,
    "avg_savings_vs_direct_api": 22
  }
}
```

**Authentication**: None (public endpoint)

---

### GET /api/v1/services/:id/reviews

**Purpose**: Get reviews for a service

**Query Parameters**:
- `limit`: number (default: 10, max: 100)
- `sort`: `recent` | `rating_desc` | `rating_asc` (default: `recent`)

**Response**:
```json
{
  "service_id": 789,
  "avg_rating": 4.6,
  "total_reviews": 89,
  "rating_distribution": {
    "5": 56,
    "4": 25,
    "3": 5,
    "2": 2,
    "1": 1
  },
  "reviews": [
    {
      "id": 1234,
      "rating": 5,
      "comment": "Fast and reliable service!",
      "reviewer": "user_***abc",  // anonymized
      "created_at": "2025-11-20T14:30:00Z",
      "verified_purchase": true
    }
  ]
}
```

---

### POST /api/v1/services/:id/reviews

**Purpose**: Submit a review for a service

**Request Body**:
```json
{
  "rating": 5,
  "comment": "Excellent service, highly recommend!"
}
```

**Response**:
```json
{
  "review": {
    "id": 1235,
    "service_id": 789,
    "rating": 5,
    "comment": "Excellent service, highly recommend!",
    "created_at": "2025-11-22T18:00:00Z"
  }
}
```

**Requirements**:
- User must be an active subscriber to leave a review
- One review per user per service
- Reviews can be updated (PATCH /api/v1/reviews/:id)

---

## 7. Billing & Settlement (Consumer)

### GET /api/v1/billing/current

**Purpose**: Get current billing cycle summary

**Response**:
```json
{
  "billing_cycle": {
    "start": "2025-11-01T00:00:00Z",
    "end": "2025-11-30T23:59:59Z",
    "due_date": "2025-12-01T00:00:00Z"
  },
  "summary": {
    "total_tokens_consumed": 450000,
    "total_cost_before_fees": 129.38,
    "platform_fee": 10.92,
    "sales_tax": 12.01,
    "total_due": 142.30
  },
  "by_service": [
    {
      "service_id": 456,
      "service_name": "GPT-4o Official",
      "tokens_consumed": 180000,
      "cost": 45.00
    },
    {
      "service_id": 457,
      "service_name": "Claude Sonnet",
      "tokens_consumed": 240000,
      "cost": 36.00
    }
  ],
  "payment_method": {
    "type": "card",
    "card_last_4": "4242",
    "card_brand": "Visa"
  }
}
```

---

### GET /api/v1/billing/history

**Purpose**: Get billing history

**Query Parameters**:
- `limit`: number (default: 12, max: 100)

**Response**:
```json
{
  "invoices": [
    {
      "id": "inv_202511",
      "billing_period": "2025-11",
      "amount": 142.30,
      "status": "paid",
      "paid_at": "2025-12-01T10:00:00Z",
      "invoice_url": "https://..."
    },
    {
      "id": "inv_202510",
      "billing_period": "2025-10",
      "amount": 98.50,
      "status": "paid",
      "paid_at": "2025-11-01T09:30:00Z",
      "invoice_url": "https://..."
    }
  ]
}
```

---

### POST /api/v1/billing/payment-method

**Purpose**: Add or update payment method

**Request Body**:
```json
{
  "type": "card",
  "card_token": "tok_visa_4242"  // Stripe token or similar
}
```

**Response**:
```json
{
  "payment_method": {
    "id": "pm_123456",
    "type": "card",
    "card_last_4": "4242",
    "card_brand": "Visa",
    "default": true
  }
}
```

---

## 8. Service Discovery Enhancements

### GET /api/v1/services (Enhanced)

**Purpose**: Enhanced service listing with filters and sorting

**Query Parameters**:
- `scope`: `all` | `mine` (default: `all`)
- `model_family`: filter by model (e.g., `gpt-4o`, `claude-3`)
- `max_price`: max price per 1K tokens (e.g., `0.025`)
- `min_rating`: minimum average rating (e.g., `4.5`)
- `sort`: `price_asc` | `price_desc` | `rating` | `popularity` (default: `price_asc`)
- `has_trial`: `true` | `false` (only services with free trials)
- `limit`: number (default: 20, max: 100)
- `offset`: number (for pagination)

**Response**: Same as existing `/api/v1/services` but with additional metadata:
```json
{
  "services": [
    {
      "id": 456,
      "provider_id": 12,
      "name": "GPT-4o Budget",
      "model_family": "gpt-4o",
      "price_per_1k_tokens": 0.021,
      "trial_tokens": 10000,
      "avg_rating": 4.7,
      "review_count": 1234,
      "subscriber_count": 5678,
      "uptime_percentage": 99.5,
      "verified_provider": true
    }
  ],
  "pagination": {
    "total": 245,
    "limit": 20,
    "offset": 0,
    "has_more": true
  }
}
```

---

## Implementation Priority

### Phase 1: Core Marketplace (High Priority)
1. ✅ Service subscription endpoints (buy tokens)
2. ✅ Service publishing endpoints (sell tokens)
3. ✅ Enhanced service discovery with filters
4. ✅ Basic revenue summary

### Phase 2: Revenue & Analytics (Medium Priority)
5. ✅ Revenue trends API
6. ✅ Top customers API
7. ✅ Payout method management
8. ✅ Billing current/history

### Phase 3: Social & Trust (Lower Priority)
9. Reviews and ratings
10. Marketplace statistics
11. Provider verification badges

### Phase 4: Advanced Features (Future)
12. Edition detection API
13. Dynamic pricing recommendations
14. Fraud detection
15. Chargebacks and disputes

---

## Notes for Backend Implementation

1. **Platform Fee Configuration**:
   - Default: 10% of provider revenue
   - Should be configurable via environment variable
   - Consider volume-based tiers (e.g., 10% → 8% for >1M tokens/month)

2. **Billing Cycles**:
   - Monthly billing on the 1st of each month
   - Grace period: 7 days
   - Auto-pause services after 30 days non-payment

3. **Payout Schedule**:
   - Payouts on the 1st of each month
   - Minimum payout threshold: $50
   - Hold period: 7 days for new providers (fraud prevention)

4. **Rate Limiting**:
   - Service publishing: 10 services per provider
   - Review submission: 1 review per service per user
   - API calls: Standard rate limits apply

5. **Data Privacy**:
   - Anonymize customer data in provider analytics
   - GDPR/CCPA compliance for user data
   - Encrypt payment information

---

**End of Document**
