# Backend CI/CD

This repository now includes two GitHub Actions workflows:

- `Backend CI`: runs tests, race checks, `go vet`, `golangci-lint`, `gosec`, `govulncheck`, Compose validation, and Docker image build checks.
- `Backend CD`: builds and pushes production Docker images to GHCR, then deploys them over SSH with Docker Compose.

## What you need in GitHub

Add these repository or environment secrets before enabling production deploy:

- `DEPLOY_HOST`
- `DEPLOY_PORT`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_PATH`
- `AUTH_DATABASE_URL`
- `GROUP_DATABASE_URL`
- `POSTGRES_DB`
- `POSTGRES_USERNAME`
- `POSTGRES_PASSWORD`
- `GROUP_POSTGRES_DB`
- `GROUP_POSTGRES_USERNAME`
- `GROUP_POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `RABBITMQ_USERNAME`
- `RABBITMQ_PASSWORD`
- `JWT_KEY_ID`
- `JWT_PUBLIC_KEYS`
- `JWT_REFRESH_TTL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `EMAIL_FROM`
- `LOG_LEVEL`
- `ENVOY_LOG_LEVEL`
- `CORS_ALLOWED_ORIGIN_REGEX`

## Files that must exist on the server

The deploy workflow expects these files to already exist on the target host:

- `${DEPLOY_PATH}/auth-service/keys/jwt-private.pem`
- `${DEPLOY_PATH}/auth-service/keys/jwt-public.pem`
- `${DEPLOY_PATH}/api-gateway/certs/fullchain.pem`
- `${DEPLOY_PATH}/api-gateway/certs/privkey.pem`

The workflow syncs:

- `compose.yaml`
- `deploy/docker-compose.production.yaml`

## Server bootstrap

Prepare the VPS once:

```bash
mkdir -p /opt/money-tracking/auth-service/keys
mkdir -p /opt/money-tracking/api-gateway/certs
```

Copy your JWT and TLS files into those folders, then point `DEPLOY_PATH` at that directory.

## Deploy behavior

- Every push to `main` publishes four images to GHCR.
- The deploy job pulls the images tagged with the current commit SHA.
- `workflow_dispatch` lets you re-run deployment manually.
