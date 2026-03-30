#!/bin/sh
# Используем /bin/sh вместо /bin/bash для лучшей совместимости

# Конфигурация
TOKEN_FILE="/etc/prometheus/minio-token"
MINIO_CONTAINER=${MINIO_CONTAINER:-"pigeongram_minio"}
MINIO_ENDPOINT=${MINIO_ENDPOINT:-"http://minio:9000"}
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-"minioadmin"}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-"minioadmin"}
PROMETHEUS_URL=${PROMETHEUS_URL:-"http://prometheus:9090"}

# Функция для логирования
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Получение токена из MinIO
get_minio_token() {
    log "🔄 Генерация нового токена MinIO..."
    
    # Проверяем, доступен ли docker
    if ! command -v docker >/dev/null 2>&1; then
        log "❌ docker не найден"
        return 1
    fi
    
    # Проверяем, запущен ли контейнер MinIO
    if ! docker ps | grep -q "$MINIO_CONTAINER"; then
        log "❌ Контейнер $MINIO_CONTAINER не запущен"
        return 1
    fi
    
    # Настраиваем алиас в MinIO контейнере
    docker exec $MINIO_CONTAINER mc alias set myminio $MINIO_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY > /dev/null 2>&1
    
    # Генерируем конфигурацию Prometheus и извлекаем токен
    TOKEN=$(docker exec $MINIO_CONTAINER mc admin prometheus generate myminio 2>/dev/null | grep "bearer_token" | awk -F': ' '{print $2}' | tr -d '"' | tr -d '\r')
    
    if [ -z "$TOKEN" ]; then
        log "❌ Не удалось получить токен"
        return 1
    fi
    
    log "✅ Токен успешно получен"
    echo "$TOKEN"
    return 0
}

# Сохранение токена в файл
save_token() {
    local token=$1
    local temp_file="${TOKEN_FILE}.tmp"
    
    # Создаем директорию если её нет
    mkdir -p $(dirname $TOKEN_FILE)
    
    # Записываем во временный файл
    echo "$token" > $temp_file
    
    # Атомарно перемещаем
    mv $temp_file $TOKEN_FILE
    
    log "💾 Токен сохранен в $TOKEN_FILE"
    
    # Устанавливаем правильные права
    chmod 644 $TOKEN_FILE
}

# Перезагрузка Prometheus
reload_prometheus() {
    log "🔄 Перезагрузка Prometheus..."
    
    # Пробуем перезагрузить Prometheus через API
    if command -v curl >/dev/null 2>&1; then
        curl -s -X POST $PROMETHEUS_URL/-/reload || true
    fi
    
    log "✅ Запрос на перезагрузку отправлен"
}

# Основная функция
main() {
    # Получаем новый токен
    TOKEN=$(get_minio_token)
    if [ $? -eq 0 ] && [ ! -z "$TOKEN" ]; then
        save_token "$TOKEN"
        reload_prometheus
        log "✅ Токен обновлен и применен"
    else
        log "❌ Ошибка получения токена"
        exit 1
    fi
}

# Запуск
main