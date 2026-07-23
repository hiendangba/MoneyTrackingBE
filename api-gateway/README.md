# API Gateway

Envoy is the only public backend entrypoint. It verifies five-minute RS256 access tokens locally and routes requests to the private `auth-service`.

## Security model

- `Authorization: Bearer` takes precedence over the `access_token` cookie.
- Lua removes client-supplied identity headers before JWT verification.
- Envoy accepts JWTs only from the Authorization header, not `?access_token=`.
- Public routes are allowlisted. The final `/` rule fails closed.
- RBAC requires the verified claim `token_type=access` on protected routes.
- Refresh tokens are never accepted as API credentials.
- Envoy fetches public keys from `/.well-known/jwks.json`; private keys never leave `auth-service`.

Browser flows first call `GET /api/auth/csrf`, retain the returned cookie, and send the same value in `X-CSRF-Token` on mutations. Mobile clients use `/api/auth/mobile/*` with JSON tokens and do not use CSRF cookies.

## Local Compose

Create `.env` from `.env.example`, provide JWT keys and local TLS files, then run:

```powershell
docker compose up --build
```

Public URLs are `http://localhost` (308 redirect) and `https://localhost`. Direct infrastructure ports are disabled. For localhost-only diagnostics:

```powershell
docker compose -f compose.yaml -f compose.debug.yaml up --build
```

Local Envoy admin binds to `127.0.0.1` inside its container and is not published. Inspect it with `docker compose exec api-gateway wget -qO- http://127.0.0.1:9901/server_info`.

## TLS

Local standalone Envoy expects:

- `api-gateway/certs/fullchain.pem`
- `api-gateway/certs/privkey.pem`

The existing `manage-letsencrypt.ps1` helper can create a local self-signed pair or issue/renew Let's Encrypt certificates after a real domain points to the host.

Render terminates trusted HTTPS before Envoy. Set `ENVOY_RENDER_MODE=true`; the entrypoint renders `envoy.render.yaml.template` with `${PORT}`, private auth host/port, and the CORS origin. Do not mount TLS certificates or redirect Render's internal HTTP listener.

## Render secrets

The root `render.yaml` is a compatibility sample, not an instruction to provision paid services. In the private auth service's Render dashboard, add these secret files:

- `/etc/secrets/jwt-private.pem`
- `/etc/secrets/jwt-public.pem`

During key rotation, keep old public keys in `JWT_PUBLIC_KEYS` until every token signed by them has expired, while `JWT_KEY_ID` points to the active private key.
