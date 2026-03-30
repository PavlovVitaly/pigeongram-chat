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

# Переходим в корневую директорию проекта
cd /opt/pigeongram

# 1. Проверка .env.production
if [ ! -f "deploy/config/.env.production" ]; then
    print_error "deploy/config/.env.production не найден!"
    exit 1
fi

# 2. Загрузка переменных окружения
source deploy/config/.env.production

# 3. Сборка Docker образа приложения
print_step "Сборка Docker образа приложения"
cd app
docker build -t pigeongram:latest .
cd ..

# 4. Создание необходимых директорий
print_step "Создание директорий"
mkdir -p deploy/backups
mkdir -p deploy/logs/app

# 5. Проверка конфигурационных файлов
print_step "Проверка конфигурационных файлов"

# Проверка nginx.conf
if [ ! -f "deploy/config/nginx/nginx.conf" ]; then
    print_error "deploy/config/nginx/nginx.conf не найден!"
    exit 1
fi

# Проверка prometheus.yml
if [ ! -f "deploy/monitoring/prometheus.yml" ]; then
    print_error "deploy/monitoring/prometheus.yml не найден!"
    exit 1
fi

# Проверка alerts.yml
if [ ! -f "deploy/monitoring/alerts.yml" ]; then
    print_error "deploy/monitoring/alerts.yml не найден!"
    exit 1
fi

# 6. Остановка старых контейнеров (если есть)
print_step "Остановка старых контейнеров"
cd deploy
docker-compose down 2>/dev/null || true

# 7. Запуск всех сервисов
print_step "Запуск всех сервисов"
docker-compose up -d

# 8. Ожидание готовности сервисов
print_step "Ожидание запуска сервисов..."
sleep 15

# 9. Инициализация MinIO (создание bucket)
print_step "Инициализация MinIO"
chmod +x /opt/pigeongram/deploy/scripts/init-minio.sh
/opt/pigeongram/deploy/scripts/init-minio.sh

# 10. Проверка статуса
print_step "Проверка статуса контейнеров"
echo ""
docker-compose ps

# 11. Проверка доступности
print_step "Проверка доступности сервисов"

# Проверка health endpoint приложения
if curl -s http://localhost/health > /dev/null 2>&1; then
    print_success "Приложение доступно: http://localhost/health"
else
    print_error "Приложение недоступно"
fi

# Проверка Prometheus
if curl -s http://localhost/prometheus/-/healthy > /dev/null 2>&1; then
    print_success "Prometheus доступен: http://localhost/prometheus"
else
    print_warning "Prometheus недоступен"
fi

# Проверка Grafana
if curl -s http://localhost/grafana/api/health > /dev/null 2>&1; then
    print_success "Grafana доступна: http://localhost/grafana"
else
    print_warning "Grafana недоступна"
fi

cd ..

print_success "Развертывание завершено!"
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "🔗 Доступ к сервисам:"
echo "═══════════════════════════════════════════════════════════════"
echo "   Чат:           http://${DOMAIN:-45.8.97.91}/chat"
echo "   Файлы:         http://${DOMAIN:-45.8.97.91}/files?chat_id=general"
echo "   MinIO Console: http://${DOMAIN:-45.8.97.91}/minio-console/"
echo "   Prometheus:    http://${DOMAIN:-45.8.97.91}/prometheus/"
echo "   Grafana:       http://${DOMAIN:-45.8.97.91}/grafana/ (admin/admin)"
echo ""
echo "📊 Команды для управления:"
echo "   cd /opt/pigeongram/deploy"
echo "   docker-compose ps         # статус контейнеров"
echo "   docker-compose logs -f    # просмотр логов"
echo "   docker-compose down       # остановка всех сервисов"
echo "   docker-compose up -d      # запуск всех сервисов"
echo "═══════════════════════════════════════════════════════════════"