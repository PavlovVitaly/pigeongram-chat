# Этап 1: Сборка
FROM golang:1.25-alpine AS builder

# Устанавливаем git для go mod
RUN apk add --no-cache git ca-certificates

# Рабочая директория
WORKDIR /app

# Копируем файлы модулей
COPY go.mod go.sum* ./

# Загружаем зависимости
RUN go mod download -x

# Копируем весь исходный код
COPY . .

# Собираем приложение (main.go в корне)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /app/bin/pigeongram

# Этап 2: Финальный образ
FROM alpine:latest

# Добавляем CA сертификаты для HTTPS и tzdata
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# Создаем непривилегированного пользователя
RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

# Рабочая директория
WORKDIR /app

# Копируем бинарник
COPY --from=builder --chown=app:app /app/bin/pigeongram /app/
COPY --from=builder --chown=app:app /app/web /app/web/

# Проверяем существует ли папка web и копируем её, если есть
RUN if [ -d /app/web ]; then \
        mkdir -p /app/web && \
        cp -r /app/web/* /app/web/ 2>/dev/null || true; \
    fi

# Создаем директорию для логов
RUN mkdir -p /app/logs && chown -R app:app /app/logs

# Переключаемся на непривилегированного пользователя
USER app

# Порт приложения
EXPOSE 8080

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Запуск
CMD ["./pigeongram"]