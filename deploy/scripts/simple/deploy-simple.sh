#!/bin/bash

# =====================================================
# PigeonGram - Простое развертывание
# =====================================================

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_step() { echo -e "\n${BLUE}▶ $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }
print_info() { echo -e "${YELLOW}ℹ️ $1${NC}"; }

APP_DIR="/opt/pigeongram"
ENV_FILE="$APP_DIR/config/.env.production"

# =====================================================
# 1. Проверка окружения
# =====================================================
print_step "Проверка окружения"

if [ ! -f "$ENV_FILE" ]; then
    print_error ".env.production не найден в $APP_DIR/config/"
    exit 1
fi

# =====================================================
# 2. Запуск инфраструктуры через docker-compose
# =====================================================
print_step "Запуск инфраструктуры (PostgreSQL, Redis, MinIO, Nginx)"

cd "$APP_DIR/repo/docker/postgres"

# Остановить всё, если нужно
docker-compose down 2>/dev/null || true

# Запустить всё одной командой
docker-compose up -d

print_success "Инфраструктура запущена"

# =====================================================
# 3. Ожидание готовности MinIO
# =====================================================
print_step "Ожидание готовности MinIO"

for i in {1..30}; do
    if docker exec pigeongram_minio curl -s http://localhost:9000/minio/health/live 2>/dev/null; then
        print_success "MinIO готов"
        break
    fi
    sleep 2
done

# =====================================================
# 4. Получение IP контейнеров для static hosts
# =====================================================
print_step "Получение IP контейнеров"

NGINX_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_nginx 2>/dev/null)
MINIO_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_minio 2>/dev/null)
POSTGRES_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_postgres 2>/dev/null)
REDIS_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_redis 2>/dev/null)

print_info "NGINX IP: $NGINX_IP"
print_info "MINIO IP: $MINIO_IP"
print_info "POSTGRES IP: $POSTGRES_IP"
print_info "REDIS IP: $REDIS_IP"

# =====================================================
# 5. Сборка и запуск приложения
# =====================================================
print_step "Сборка и запуск приложения"

cd "$APP_DIR/repo"

# Собрать образ
docker build -t pigeongram:latest .

# Удалить старый контейнер если есть
docker stop pigeongram_app 2>/dev/null || true
docker rm pigeongram_app 2>/dev/null || true

# Запустить приложение
docker run -d \
    --name pigeongram_app \
    --restart unless-stopped \
    -p 8080:8080 \
    --network pigeongram_network \
    -v "$APP_DIR/logs":/app/logs \
    --env-file "$APP_DIR/config/.env.production" \
    --add-host "nginx:$NGINX_IP" \
    --add-host "minio:$MINIO_IP" \
    --add-host "postgres:$POSTGRES_IP" \
    --add-host "redis:$REDIS_IP" \
    pigeongram:latest

print_success "Приложение запущено"

# =====================================================
# 6. Ожидание запуска приложения
# =====================================================
print_step "Ожидание запуска приложения"

for i in {1..30}; do
    if docker ps | grep -q "pigeongram_app.*Up"; then
        print_success "Приложение работает"
        break
    fi
    sleep 2
done

# =====================================================
# 7. Финальная проверка
# =====================================================
print_step "Проверка работы"

sleep 5

# Проверить логи приложения
docker logs pigeongram_app 2>&1 | grep -E "(MinIO|Файловое)" | tail -5

# Проверить страницы
echo ""
curl -s -o /dev/null -w "Чат: %{http_code}\n" http://92.255.108.89/chat
curl -s -o /dev/null -w "Файлы: %{http_code}\n" http://92.255.108.89/files?chat_id=general
curl -s -o /dev/null -w "Стили: %{http_code}\n" http://92.255.108.89/static/style.css

echo ""
print_success "Развертывание завершено!"
print_info "Чат: http://92.255.108.89/chat"
print_info "Файлы: http://92.255.108.89/files?chat_id=general"