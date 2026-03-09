# syntax=docker/dockerfile:1.4
FROM golang:1-alpine AS builder
#FROM golang:1.26rc3-alpine AS builder

# устанавливаем рабочую директорию
WORKDIR /app

# зависимости для сборки
RUN apk add --no-cache git ca-certificates

# копируем go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем зависимости с кэшированием через BuildKit
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# копируем исходный код
COPY . .

# Собираем бинарник с кэшированием
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o app ./cmd/myapp/main.go

# запускаем на alpine
FROM alpine:latest

# сертификаты для htpp запросов
RUN apk add --no-cache ca-certificates

# копируем бинарник из builder
COPY --from=builder /app/app .

# копируем папку с ресурсами
COPY --from=builder /app/assets ./assets

# копируем папку с миграциями
COPY --from=builder /app/migrations ./migrations

# копируем .env файл
COPY --from=builder /app/.env .

CMD ["./app"]
