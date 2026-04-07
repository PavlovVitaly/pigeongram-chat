#!/bin/bash
# =====================================================
# PigeonGram - Инициализация MinIO
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

# Загрузка переменных окружения
if [ -f "/opt/pigeongram/deploy/config/.env.production" ]; then
    source /opt/pigeongram/deploy/config/.env.production
fi

# Настройки по умолчанию
MINIO_CONTAINER=${MINIO_CONTAINER:-pigeongram_minio}
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-minioadmin}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-minioadmin}
MINIO_BUCKET=${MINIO_BUCKET:-pigeongram-files}
MINIO_PUBLIC_URL=${MINIO_PUBLIC_URL:-"https://45.8.97.91"}

print_step "Ожидание запуска MinIO..."

# Ждем пока MinIO запустится
for i in {1..30}; do
    if docker exec $MINIO_CONTAINER curl -s http://localhost:9000/minio/health/live > /dev/null 2>&1; then
        print_success "MinIO готов"
        break
    fi
    echo -n "."
    sleep 2
done

# Настройка mc alias
print_step "Настройка mc alias"
docker exec $MINIO_CONTAINER mc alias set local http://localhost:9000 $MINIO_ACCESS_KEY $MINIO_SECRET_KEY 2>/dev/null || true

# Создание bucket
print_step "Создание bucket '$MINIO_BUCKET'"
docker exec $MINIO_CONTAINER mc mb local/$MINIO_BUCKET --ignore-existing 2>/dev/null || true

# Установка публичного доступа (опционально)
# docker exec $MINIO_CONTAINER mc anonymous set download local/$MINIO_BUCKET

print_success "MinIO инициализирован"
print_info "Bucket: $MINIO_BUCKET"
print_info "Access Key: $MINIO_ACCESS_KEY"