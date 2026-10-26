FROM golang:1.27.3-alpine AS builder

WORKDIR /src

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build \
  -trimpath \
  -ldflags='-s -w' \
  -o /out/proxymaster ./cmd/telegram


FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/proxymaster /app/proxymaster
COPY --from=builder /src/assets /app/assets

USER app
ENTRYPOINT ["/app/proxymaster"]
