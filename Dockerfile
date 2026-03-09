FROM golang:1.25.7-alpine AS builder
#FROM golang:1.26rc3-alpine AS builder

# устанавливаем рабочую директорию
WORKDIR /app

# зависимости для сборки
RUN apk add --no-cache git ca-certificates

# Директория для кеша сборок
RUN mkdir -p /go-cache

ENV GOCACHE=/go-cache/build-cache
ENV GOMODCACHE=/go-cache/mod-cache

# копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# копируем исходный код
COPY . .

# собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o app ./cmd/myapp/main.go

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
