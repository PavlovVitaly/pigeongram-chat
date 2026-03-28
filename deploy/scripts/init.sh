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
docker-compose up -d postgres redis minio

# 5. Ожидание готовности
print_step "Ожидание запуска сервисов..."
sleep 10

# 6. Инициализация MinIO
print_step "Инициализация MinIO"
chmod +x init-scripts/init-minio.sh
./init-scripts/init-minio.sh

# 7. Запуск приложения
print_step "Запуск приложения"
docker-compose up -d app

# 8. Ожидание запуска приложения
sleep 10

# 9. Проверка статуса
print_step "Проверка статуса"
docker-compose ps

# 10. Проверка страницы файлов
print_step "Проверка страницы файлов"
curl -s -o /dev/null -w "Страница файлов: %{http_code}\n" http://92.255.108.89/files?chat_id=general

print_success "Развертывание завершено!"
print_info "Чат: http://92.255.108.89/chat"
print_info "Файлы: http://92.255.108.89/files?chat_id=general"
print_info "MinIO Console: http://92.255.108.89:9001 (minioadmin/minioadmin)"