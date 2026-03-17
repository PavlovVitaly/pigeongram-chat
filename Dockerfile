# Этап 1: Сборка
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Копируем модули
COPY go.mod go.sum* ./

# Загружаем зависимости
RUN --mount=type=cache,target=/go/pkg/mod \
    if [ -f go.sum ]; then \
        go mod download -x; \
    else \
        go mod download -x || (sleep 5 && go mod download -x); \
    fi

# Копируем исходный код
COPY . .

# Определяем точку входа (main.go может быть в корне или в cmd/server)
RUN if [ -f ./cmd/server/main.go ]; then \
        CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /app/bin/pigeongram ./cmd/server; \
    elif [ -f ./main.go ]; then \
        CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /app/bin/pigeongram .; \
    else \
        echo "main.go not found" && exit 1; \
    fi

# Этап 2: Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates && \
    addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

WORKDIR /app

COPY --from=builder --chown=app:app /app/bin/pigeongram /app/
COPY --from=builder --chown=app:app /app/web /app/web

RUN mkdir -p /app/logs && chown -R app:app /app/logs

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

CMD ["./pigeongram"]