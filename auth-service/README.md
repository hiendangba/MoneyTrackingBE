# Auth Service

Go authentication and global menu service for MoneyTracking.

## Authentication contracts

- Access tokens use audience `money-tracking-api` and expire after five minutes.
- Refresh tokens use audience `money-tracking-refresh`, rotate once, and belong to a `sid` session family.
- Required claims are `exp`, `iat`, `nbf`, `sub`, `jti`, `sid`, `session_version`, `aud`, and `token_type`.
- Password reset/change increments `users.session_version` atomically.
- Refresh replay revokes the entire session family in Redis.
- Browser endpoints use Secure HttpOnly cookies plus Origin and double-submit CSRF validation.
- Mobile endpoints return Bearer tokens in JSON and never set cookies.

Browser clients call `GET /api/auth/csrf` before mutations. Mobile endpoints are:

- `POST /api/auth/mobile/login`
- `POST /api/auth/mobile/refresh`
- `POST /api/auth/mobile/logout`

JWKS is public at `GET /.well-known/jwks.json` and contains public keys only.

## JWT key rotation

`JWT_PRIVATE_KEY_PATH` contains the active private key. `JWT_KEY_ID` is its `kid`. `JWT_PUBLIC_KEYS` is a semicolon-separated key ring:

```text
active-kid=/app/keys/jwt-public.pem;previous-kid=/app/keys/jwt-public-previous.pem
```

Every RSA key must be at least 2048 bits, and the active public/private pair is checked at startup.

Generate a pair with OpenSSL:

```powershell
openssl genrsa -out jwt-private.pem 3072
openssl rsa -in jwt-private.pem -pubout -out jwt-public.pem
```

Never commit private keys. On Render, upload them as secret files under `/etc/secrets`.

## Local run

Use the root `.env.example` and Compose. Direct service access is intentionally disabled unless `compose.debug.yaml` is included.

Run checks from `auth-service`:

```powershell
go test ./...
go test -race ./...
go vet ./...
```
