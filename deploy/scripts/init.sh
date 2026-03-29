#!/bin/bash
# =====================================================
# PigeonGram - Полное автоматическое развертывание
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

cd /opt/pigeongram

# 1. Проверка .env
if [ ! -f ".env.production" ]; then
    print_error ".env.production не найден!"
    exit 1
fi

# 2. Загрузка переменных
source .env.production

# 3. Сборка приложения
print_step "Сборка приложения"
cd repo
docker build -t pigeongram:latest .
cd ..

# 4. Запуск инфраструктуры
print_step "Запуск инфраструктуры (PostgreSQL, Redis, MinIO)"
cd /opt/pigeongram/repo/docker/postgres/
docker-compose up -d 
cd /opt/pigeongram

# 5. Ожидание готовности
print_step "Ожидание запуска сервисов..."
sleep 15

# 6. Инициализация MinIO
print_step "Инициализация MinIO"
chmod +x init-scripts/init-minio.sh
./init-scripts/init-minio.sh

# 7. Запуск инфраструктуры
print_step "Запуск инфраструктуры для мониторинга"
cd /opt/pigeongram/repo/docker/monitoring/
docker-compose up -d 
cd /opt/pigeongram

# 8. Проверка статуса
print_step "Проверка статуса"
cd /opt/pigeongram/repo/docker/postgres/
docker-compose ps
cd /opt/pigeongram

print_success "Развертывание завершено!"
