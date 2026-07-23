FROM golang:1.26.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /auth-service ./cmd/api

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates su-exec \
    && addgroup -g 1000 -S appgroup \
    && adduser -S appuser -G appgroup

COPY --from=builder /auth-service /app/auth-service
COPY docker-entrypoint.sh /usr/local/bin/auth-service-entrypoint

RUN chmod 0555 /usr/local/bin/auth-service-entrypoint

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/auth-service-entrypoint"]
