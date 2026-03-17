#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/opt/pigeongram"

print_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}ℹ️ $1${NC}"
}

# Загружаем конфигурацию (включая GitHub токен)
if [ -f "$PROJECT_ROOT/config/.env.production" ]; then
    source "$PROJECT_ROOT/config/.env.production"
else
    print_error "Файл .env.production не найден в $PROJECT_ROOT/config/"
    exit 1
fi

# GitHub репозиторий из конфигурации
GIT_REPO="${GITHUB_URL}"

print_step "Начало деплоя PigeonGram"
print_info "Репозиторий: https://github.com/${GITHUB_USER}/${GITHUB_REPO}"

# 1. Загрузка исходного кода
print_step "Клонирование/обновление репозитория"
cd $PROJECT_ROOT
if [ -d "repo" ]; then
    cd repo
    print_info "Обновление существующего репозитория..."
    git remote set-url origin $GIT_REPO
    git pull origin main
    print_success "Репозиторий обновлен"
else
    print_info "Клонирование репозитория..."
    git clone $GIT_REPO repo
    cd repo
    print_success "Репозиторий склонирован"
fi

# 2. Копирование конфигурации
print_step "Настройка конфигурации"
cp $PROJECT_ROOT/config/.env.production $PROJECT_ROOT/repo/.env
print_success "Конфигурация скопирована"

# 3. Генерация SSL сертификатов
print_step "Генерация SSL сертификатов"
if [ ! -f "$PROJECT_ROOT/ssl/cert.pem" ]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout $PROJECT_ROOT/ssl/key.pem \
        -out $PROJECT_ROOT/ssl/cert.pem \
        -subj "/C=RU/ST=Moscow/L=Moscow/O=PigeonGram/CN=${DOMAIN:-localhost}"
    print_success "SSL сертификаты сгенерированы"
else
    print_warning "SSL сертификаты уже существуют"
fi

# 4. Запуск инфраструктуры через manage.sh
print_step "Запуск PostgreSQL и MinIO"
cd $PROJECT_ROOT/repo/docker/postgres
chmod +x manage.sh
./manage.sh start
print_success "PostgreSQL и MinIO запущены через manage.sh"

# 5. Запуск мониторинга
print_step "Запуск мониторинга"
cd $PROJECT_ROOT/repo/docker/monitoring

# Создаем файл для токена MinIO
touch minio-token
chmod 666 minio-token

# Запускаем мониторинг
if [ -f "monitor.sh" ]; then
    chmod +x monitor.sh
    ./monitor.sh start
else
    docker-compose up -d
fi
print_success "Мониторинг запущен"

# 6. Сборка и запуск приложения
print_step "Сборка и запуск приложения"
cd $PROJECT_ROOT/repo

# Создаем Docker сеть если её нет
docker network inspect pigeongram_network >/dev/null 2>&1 || \
    docker network create pigeongram_network

# Собираем Docker образ
docker build -t pigeongram:latest .

# Запускаем контейнер
docker run -d \
    --name pigeongram_app \
    --restart unless-stopped \
    -p 8080:8080 \
    --network pigeongram_network \
    -v $PROJECT_ROOT/logs:/app/logs \
    --env-file $PROJECT_ROOT/config/.env.production \
    pigeongram:latest

print_success "Приложение запущено"

# 7. Настройка автоматического обновления
print_step "Настройка автоматического обновления"
cat > /etc/cron.d/pigeongram-update << EOF
# Автоматическое обновление PigeonGram каждую ночь в 3:00
0 3 * * * root $PROJECT_ROOT/scripts/03-update-app.sh >> /var/log/pigeongram-update.log 2>&1
EOF
chmod 644 /etc/cron.d/pigeongram-update
print_success "Автоматическое обновление настроено"

# 8. Настройка универсального менеджера
print_step "Настройка универсального менеджера"
cd $PROJECT_ROOT/scripts
cp $PROJECT_ROOT/repo/deploy/scripts/07-manage.sh $PROJECT_ROOT/scripts/
chmod +x 07-manage.sh
ln -sf $PROJECT_ROOT/scripts/07-manage.sh /usr/local/bin/pigeongram
print_success "Универсальный менеджер установлен (команда 'pigeongram')"

# 9. Информация о запущенных сервисах
print_step "Проверка статуса"
echo ""
echo "📊 Статус всех сервисов:"
$PROJECT_ROOT/scripts/07-manage.sh status

echo ""
echo "🌐 Доступные сервисы:"
SERVER_IP=$(curl -s ifconfig.me)
echo "   Приложение: http://$SERVER_IP:8080"
echo "   PostgreSQL: доступен через Docker сеть"
echo "   MinIO API:  http://$SERVER_IP:9000"
echo "   MinIO Web:  http://$SERVER_IP:9001 (minioadmin/пароль из .env)"
echo "   Prometheus: http://$SERVER_IP:9090"
echo "   Grafana:    http://$SERVER_IP:3000 (admin/admin)"

echo ""
echo "📋 Управление проектом:"
echo "   pigeonram status        - статус всех сервисов"
echo "   pigeonram start         - запустить все"
echo "   pigeonram stop          - остановить все"
echo "   pigeonram backup        - создать бэкап"
echo "   pigeonram logs app      - логи приложения"
echo "   pigeonram postgres help - управление БД"
echo "   pigeonram monitor help  - управление мониторингом"

print_success "Деплой завершен успешно!"