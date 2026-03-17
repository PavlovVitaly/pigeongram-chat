#!/bin/bash

PROJECT_ROOT="/opt/pigeongram"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_step "Начало обновления PigeonGram"

# Создаем бэкап перед обновлением
print_step "Создание резервной копии"
$PROJECT_ROOT/scripts/04-backup.sh

# Обновляем код
print_step "Обновление кода из репозитория"
cd $PROJECT_ROOT/repo
git pull
print_success "Код обновлен"

# Пересобираем приложение
print_step "Пересборка Docker образа"
docker stop pigeongram_app 2>/dev/null
docker rm pigeongram_app 2>/dev/null
docker build -t pigeongram:latest .
print_success "Образ собран"

# Запускаем новую версию
print_step "Запуск новой версии"
docker run -d \
    --name pigeongram_app \
    --restart unless-stopped \
    -p 8080:8080 \
    --network pigeongram_network \
    -v $PROJECT_ROOT/logs:/app/logs \
    --env-file $PROJECT_ROOT/config/.env.production \
    pigeongram:latest
print_success "Новая версия запущена"

# Очищаем старые образы
print_step "Очистка старых образов"
docker system prune -f
print_success "Очистка завершена"

print_success "PigeonGram обновлен до последней версии"