# syntax=docker/dockerfile:1.4

FROM golang:1.26.1-alpine AS builder

WORKDIR /app

# зависимости для сборки
RUN apk add --no-cache git ca-certificates

# сначала зависимости (для кеша)
COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# копируем исходники
COPY . .

# сборка бинарника
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o app ./cmd/myapp/main.go


# минимальный runtime образ
FROM scratch

WORKDIR /

# сертификаты для HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# бинарник
COPY --from=builder /app/app /app

# ресурсы
COPY --from=builder /app/assets /assets
COPY --from=builder /app/migrations /migrations

ENTRYPOINT ["/app"]
