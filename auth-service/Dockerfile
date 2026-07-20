FROM golang:1.26.5-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o auth-service ./cmd/api

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /app/auth-service /app/auth-service
EXPOSE 8080
CMD ["/app/auth-service"]
