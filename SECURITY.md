# Security Policy
 
## Supported Versions

Only the latest stable version of Astra is supported for security updates.

| Version | Supported          |
| ------- | ------------------ |
| v4.0.x  | :white_check_mark: |
| < v4.0  | :x:                |

## Reporting a Vulnerability

We take the security of Astra seriously. If you believe you have found a security vulnerability, please do NOT open a public issue. Instead, report it through the following process:

1. **Email us**: Send a detailed description of the vulnerability to [security@astra.dev](mailto:security@astra.dev).
2. **Response time**: You can expect an acknowledgment within 24-48 hours.
3. **Disclosure**: We will work with you to understand the scope of the issue and provide a fix. We ask that you give us reasonable time to investigate and address the report before making any information public.

## Security Practices in Astra

Astra is designed with security-first principles:
- **Transparent PII Encryption**: Native support for encrypting sensitive fields at rest.
- **CSRF Protection**: Default protection for all state-changing requests.
- **Secure Headers**: Automatic injection of CSP, HSTS, and other security headers.
- **Audit Logging**: Structured audit logs for critical operations.

Thank you for helping keep Astra safe!
