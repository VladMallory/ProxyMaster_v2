FROM golang:1.25.5-alpine AS builder

# устанавливаем рабочую директорию
WORKDIR /app

# зависимости для сборки
RUN apk add --no-cache git ca-certificates

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

CMD ["./app"]