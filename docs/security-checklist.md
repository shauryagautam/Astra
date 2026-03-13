# Security Checklist

Building secure applications is a core pillar of the Astra framework. Astra handles many common security threats out of the box, but developers must remain vigilant. Use this checklist to ensure your Astra app is production-ready.

## ✅ Request Security

- [ ] **CSRF Protection**: Ensure the CSRF middleware is enabled for all state-changing requests (`POST`, `PUT`, `DELETE`).
- [ ] **Secure Headers**: Use `http.SecureHeaders()` middleware to enable HSTS, CSP, and XSS protection.
- [ ] **Rate Limiting**: Apply rate-limiting middleware to sensitive endpoints like Login and Registration.

## ✅ Data Security

- [ ] **Sensitive Data Encryption**: Use the `orm:"encrypted"` tag for any PII (Personally Identifiable Information) stored in your database.
- [ ] **Password Hashing**: Always use Astra's `auth.HashPassword` (Argon2ID) instead of storing plain text passwords.
- [ ] **Audit Logs**: Enable audit logging for sensitive actions (e.g., changing passwords, administrative changes).

## ✅ Authentication & Authorization

- [ ] **Strong JWT Secret**: Ensure `JWT_SECRET` in your `.env` is a high-entropy string (at least 32 characters).
- [ ] **Token Expiry**: Keep `JWT_ACCESS_EXPIRY` short (e.g., 15m) and use refresh tokens for longer sessions.
- [ ] **Policy Enforcement**: Use `http.Context.Can()` or `Authorize()` to ensure users can only access their own resources.

## ✅ Infrastructure & Configuration

- [ ] **Secure Cookies**: In production, ensure all cookies are marked as `Secure` and `HttpOnly`.
- [ ] **Environment Variables**: Never commit `.env` files to version control. Use secret managers in production.
- [ ] **Debug Mode**: Ensure `APP_DEBUG=false` in production to prevent leaking sensitive stack traces.

## Astra's Hardened Defaults

By default, Astra:
- Uses **Argon2ID** for password hashing.
- Enforces **Secure/HttpOnly** cookies in production.
- Uses **Sonic JSON** for safe, fast JSON handling.
- Provides **Transparent Encryption** for database fields.
- Includes a **Centralized Audit Log** system.
