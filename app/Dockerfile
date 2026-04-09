# Этап 1: Сборка
FROM golang:1.26-alpine AS builder

# Устанавливаем git и ca-certificates
RUN apk add --no-cache git ca-certificates

# Рабочая директория
WORKDIR /app

# Копируем файлы модулей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь исходный код
COPY . .

COPY ../tmp/migration/migrate-passwords.go /app/scripts/

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /app/bin/pigeongram .

# Этап 2: Финальный образ
FROM alpine:latest

# Добавляем CA сертификаты, tzdata и curl (для healthcheck)
RUN apk --no-cache add ca-certificates tzdata curl && \
    update-ca-certificates

# Создаем непривилегированного пользователя
RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

# Рабочая директория
WORKDIR /app

# Копируем бинарник и статику
COPY --from=builder --chown=app:app /app/bin/pigeongram /app/
COPY --from=builder --chown=app:app /app/web /app/web/

# Создаем директорию для логов
RUN mkdir -p /app/logs && chown -R app:app /app/logs

# Переключаемся на непривилегированного пользователя
USER app

# Порт приложения
EXPOSE 8080

# Healthcheck (используем curl, так как он установлен)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/metrics || exit 1

# Запуск
CMD ["./pigeongram"]