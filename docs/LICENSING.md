# Licensing & Editions

Tokligence Gateway is available in two editions:

| Edition | License | Description |
|---------|---------|-------------|
| **Community** | Apache License 2.0 | Full-featured gateway for individuals and teams |
| **Enterprise** | Commercial | Advanced compliance, multi-tenancy, and enterprise features |

**Brand**: Tokligence (https://tokligence.ai). See `docs/TRADEMARKS.md` for trademark usage.

---

## Community Edition (Apache-2.0)

### What's Included
- OpenAI-compatible API (`/v1/chat/completions`, `/v1/models`, `/v1/embeddings`)
- Multi-provider adapter framework (OpenAI, Anthropic, etc.)
- Token accounting and usage ledger
- User, API key, and team management
- SQLite and PostgreSQL support
- Web UI and CLI tools
- Docker and Helm deployment examples
- Webhooks, model aliases, and routing

### License Terms
Licensed under **Apache License 2.0** (see `LICENSE` in repository root).

**Key permissions**:
- ✅ Commercial use and hosting allowed
- ✅ Modify and redistribute (with attribution)
- ✅ Patent grant included
- ✅ Build proprietary extensions and adapters

**Requirements**:
- Include `LICENSE` and `NOTICE` files in distributions
- State significant changes made to the code

---

## Enterprise Edition (Commercial)

### What's Included
Advanced features for large organizations:
- **Identity & Access**: SSO (SAML/OIDC), SCIM provisioning, RBAC
- **Multi-tenancy**: Isolated environments, cross-region deployment
- **Compliance**: GDPR/CCPA tooling, data residency, audit logs, PII redaction
- **High Availability**: Redis pools, advanced observability, SLA-backed support
- **Advanced Routing**: A/B testing, shadow traffic, circuit breakers

### How to Get It
- **Contact**: cs@tokligence.ai
- **Website**: https://tokligence.ai
- **Delivery**: License key activates enterprise modules
- **Terms**: Commercial license agreement provided upon purchase

---

## Documentation & Branding

| Asset | License | Notes |
|-------|---------|-------|
| Documentation (Markdown) | CC-BY-4.0 | See `docs/LICENSE-CC-BY-4.0` for attribution requirements |
| Brand & Logos | All Rights Reserved | See `docs/TRADEMARKS.md` for usage guidelines |

---

## Third-Party Dependencies

This project includes open-source packages under their respective licenses. See:
- `NOTICE` file for required attributions
- `go.mod` and `package.json` for dependency lists

---

## Contributing

### License for Contributions
By submitting a pull request, you agree to license your contribution under **Apache-2.0**.

You represent that:
- You have the right to license your contribution
- Your contribution does not violate third-party rights

### Contributor License Agreement (CLA)
Tokligence may request a CLA for significant contributions to streamline intellectual property management. This does not affect the Apache-2.0 license of accepted contributions.

---

## FAQ

**Can I host Community Edition as a SaaS?**
Yes. Apache-2.0 permits commercial hosting and charging for your service.

**Can I build closed-source plugins or adapters?**
Yes. Apache-2.0 allows proprietary extensions. Ensure you comply with licenses of any dependencies you import.

**Can I use "Tokligence" in my product name?**
No, not without permission. You may state "compatible with Tokligence" or "based on Tokligence Gateway" truthfully. See `docs/TRADEMARKS.md` for details.

**How do I upgrade from Community to Enterprise?**
Contact cs@tokligence.ai. Migration is seamless—Enterprise uses the same codebase and database schema.

**What about the Marketplace?**
The gateway can optionally integrate with Tokligence Token Marketplace for provider discovery and billing. Marketplace terms available at https://tokligence.ai/terms when the service launches.

---

For questions: cs@tokligence.ai
