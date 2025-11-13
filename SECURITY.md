# Security Policy

## Supported Versions

We actively support the following versions of Tokligence Gateway with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 0.3.x   | :white_check_mark: |
| < 0.3.0 | :x:                |

## Reporting a Vulnerability

The Tokligence team takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities via email to:

**[cs@tokligence.ai](mailto:cs@tokligence.ai)**

Include the following information in your report:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### What to Expect

1. **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours.

2. **Initial Assessment**: We will provide an initial assessment of the vulnerability within 5 business days, including:
   - Confirmation that we've reproduced the issue
   - Severity assessment
   - Estimated timeline for a fix

3. **Updates**: We will keep you informed of our progress as we work on a fix.

4. **Resolution**: Once a fix is ready, we will:
   - Notify you before public disclosure
   - Credit you in the security advisory (unless you prefer to remain anonymous)
   - Release a security update
   - Publish a security advisory

### Disclosure Policy

- We ask that you give us reasonable time to investigate and fix the vulnerability before any public disclosure
- We will work with you to understand and resolve the issue quickly
- We will publicly acknowledge your responsible disclosure (unless you prefer to remain anonymous)

## Security Best Practices

### For Users

When deploying Tokligence Gateway in production:

1. **API Keys**
   - Never commit API keys to version control
   - Use environment variables: `TOKLIGENCE_ANTHROPIC_API_KEY`, `TOKLIGENCE_OPENAI_API_KEY`
   - Rotate API keys regularly
   - Use separate keys for development and production

2. **Authentication**
   - Enable authentication in production (`TOKLIGENCE_AUTH_DISABLED=false`)
   - Use strong, randomly generated authentication tokens
   - Implement rate limiting at the infrastructure level

3. **Network Security**
   - Run the gateway behind a reverse proxy (nginx, Apache)
   - Use TLS/HTTPS for all external connections
   - Restrict network access using firewall rules
   - Consider using a VPN for internal services

4. **Database Security**
   - Use PostgreSQL with strong credentials in production (not SQLite)
   - Enable database encryption at rest
   - Regular database backups
   - Secure database connection strings

5. **Logging and Monitoring**
   - Enable audit logging (`TOKLIGENCE_LOG_LEVEL=info`)
   - Monitor for unusual API usage patterns
   - Set up alerts for authentication failures
   - Review logs regularly for security events

6. **Updates**
   - Keep Tokligence Gateway updated to the latest version
   - Subscribe to security advisories
   - Test updates in a staging environment first

### For Developers

When contributing to Tokligence Gateway:

1. **Input Validation**
   - Validate all user inputs
   - Sanitize data before logging
   - Use parameterized queries for database operations
   - Validate JSON payloads against schemas

2. **Dependencies**
   - Keep dependencies up to date
   - Review security advisories for dependencies
   - Use `go mod verify` to check module integrity
   - Avoid dependencies with known vulnerabilities

3. **Code Review**
   - All code changes require review
   - Security-sensitive changes require additional scrutiny
   - Use static analysis tools (`go vet`, linters)
   - Test error handling and edge cases

4. **Secrets Management**
   - Never hardcode secrets in source code
   - Never commit `.env` files or credentials
   - Use environment variables or secure vaults
   - Redact sensitive data in logs and error messages

5. **Testing**
   - Write security-focused test cases
   - Test authentication and authorization
   - Test input validation and sanitization
   - Test error handling for security implications

## Known Security Considerations

### API Key Storage

- API keys are stored in memory during runtime
- For persistent storage, use environment variables or secure secret management systems
- The gateway never logs full API keys (only last 4 characters for debugging)

### Database

- SQLite is suitable for development but consider PostgreSQL for production
- Database files should have appropriate file permissions (600)
- Enable database encryption for sensitive environments

### Session Management

- Sessions are stored in-memory only (not persisted)
- Sessions are cleared on gateway restart
- Consider session timeout policies for long-running deployments

### Tool Calling Safety

- The gateway includes infinite loop detection for tool calls
- Emergency stop triggers after 5 duplicate tool calls
- System prompts are filtered to remove references to unsupported tools

## Security Updates

Security updates will be released as patch versions (e.g., 0.3.1) and announced via:

- GitHub Security Advisories
- Release notes in `docs/releases/`
- GitHub repository notifications

## Compliance

Tokligence Gateway is designed to support:

- **GDPR**: Data sovereignty through self-hosted deployment
- **SOC 2**: Audit logging and access controls
- **HIPAA**: Encryption at rest and in transit (when properly configured)

Users are responsible for configuring the gateway appropriately for their compliance requirements.

## Contact

For security-related questions or concerns:

- **Email**: [cs@tokligence.ai](mailto:cs@tokligence.ai)
- **GitHub**: [github.com/tokligence/tokligence-gateway](https://github.com/tokligence/tokligence-gateway)

---

Thank you for helping keep Tokligence Gateway and our users safe!
