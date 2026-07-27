FROM golang:1.26.5-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY auth-service ./auth-service
COPY group-service ./group-service
COPY transaction-service ./transaction-service
COPY api-gateway ./api-gateway

WORKDIR /src/api-gateway
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/api-gateway ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /out/api-gateway /usr/local/bin/api-gateway

EXPOSE 8080

ENTRYPOINT ["api-gateway"]
