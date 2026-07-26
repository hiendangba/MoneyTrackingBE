# API Gateway

`api-gateway` là entrypoint HTTP public cho backend. Gateway nhận request từ frontend, xác thực access token, rồi gọi nội bộ `auth-service` và `group-service` bằng gRPC.

## Luồng chính

- Browser dùng cookie `access_token` + `refresh_token` và CSRF token.
- Mobile dùng `Authorization: Bearer ...` và các endpoint `/api/auth/mobile/*`.
- Gateway gọi `auth-service:50051` để verify token, login, refresh, logout, me, menu và JWKS.
- Gateway gọi `group-service:50052` cho các route group.

## Chạy local

```powershell
docker compose up --build
```

Public URL mặc định:

- `http://localhost:8080`

Gateway cần các biến môi trường chính:

- `AUTH_SERVICE_ADDR=auth-service:50051`
- `GROUP_SERVICE_ADDR=group-service:50052`
- `CORS_ALLOWED_ORIGIN_REGEX`

## Routes chính

- `GET /health`
- `GET /.well-known/jwks.json`
- `GET /api/auth/csrf`
- `POST /api/auth/login`
- `POST /api/auth/refresh-token`
- `POST /api/auth/logout`
- `POST /api/auth/mobile/login`
- `POST /api/auth/mobile/refresh`
- `POST /api/auth/mobile/logout`
- `GET /api/auth/me`
- `GET /api/menus/tree`
- `GET /api/groups`
- `POST /api/groups`

## Ghi chú

- Gateway không dùng Envoy trong luồng hiện tại.
- `auth-service` và `group-service` chỉ cần gRPC nội bộ, không cần public host port.
- `api-gateway` tự set/clear cookie cho browser flow.
