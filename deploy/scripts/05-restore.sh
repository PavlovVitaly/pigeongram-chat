#!/bin/bash

PROJECT_ROOT="/opt/pigeongram"
BACKUP_DIR="$PROJECT_ROOT/backups"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}ℹ️ $1${NC}"
}

if [ -z "$1" ]; then
    print_error "Укажите дату бэкапа (например: 20250317_143022)"
    echo ""
    echo "Доступные бэкапы:"
    ls $BACKUP_DIR/postgres_*.sql 2>/dev/null | sed 's/.*postgres_\(.*\)\.sql/\1/'
    exit 1
fi

DATE=$1

print_step "Восстановление из бэкапа от $DATE"
print_warning "ВНИМАНИЕ: Все текущие данные будут потеряны!"
read -p "Продолжить? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_info "Восстановление отменено"
    exit 0
fi

# Останавливаем все сервисы через универсальный менеджер
print_step "Остановка всех сервисов"
if [ -f "$PROJECT_ROOT/scripts/07-manage.sh" ]; then
    $PROJECT_ROOT/scripts/07-manage.sh stop
else
    docker stop pigeongram_app 2>/dev/null
    cd $PROJECT_ROOT/repo/docker/postgres && ./manage.sh stop
fi

# Восстанавливаем PostgreSQL
print_step "Восстановление PostgreSQL"
if [ -f "$BACKUP_DIR/postgres_$DATE.sql" ]; then
    cd $PROJECT_ROOT/repo/docker/postgres
    mkdir -p backups
    cp "$BACKUP_DIR/postgres_$DATE.sql" backups/
    ./manage.sh restore "backups/postgres_$DATE.sql"
    print_success "PostgreSQL восстановлен"
else
    print_warning "Бэкап PostgreSQL не найден, пропускаем"
fi

# Восстанавливаем Redis
print_step "Восстановление Redis"
if [ -f "$BACKUP_DIR/redis_$DATE.rdb" ]; then
    docker start pigeongram_redis 2>/dev/null || true
    docker cp "$BACKUP_DIR/redis_$DATE.rdb" pigeongram_redis:/data/dump.rdb
    docker restart pigeongram_redis
    print_success "Redis восстановлен"
fi

# Восстанавливаем конфигурацию
print_step "Восстановление конфигурации"
if [ -f "$BACKUP_DIR/config_$DATE.tar.gz" ]; then
    tar -xzf "$BACKUP_DIR/config_$DATE.tar.gz" -C $PROJECT_ROOT
    print_success "Конфигурация восстановлена"
fi

# Восстанавливаем логи (опционально)
print_step "Восстановление логов"
if [ -f "$BACKUP_DIR/logs_$DATE.tar.gz" ]; then
    tar -xzf "$BACKUP_DIR/logs_$DATE.tar.gz" -C $PROJECT_ROOT
    print_success "Логи восстановлены"
fi

# Запускаем все сервисы
print_step "Запуск всех сервисов"
if [ -f "$PROJECT_ROOT/scripts/07-manage.sh" ]; then
    $PROJECT_ROOT/scripts/07-manage.sh start
else
    cd $PROJECT_ROOT/repo/docker/postgres && ./manage.sh start
    cd $PROJECT_ROOT/repo/docker/monitoring && docker-compose up -d
    cd $PROJECT_ROOT/repo && docker start pigeongram_app 2>/dev/null || \
        docker run -d --name pigeongram_app --restart unless-stopped -p 8080:8080 \
        --network pigeongram_network -v $PROJECT_ROOT/logs:/app/logs \
        --env-file $PROJECT_ROOT/config/.env.production pigeongram:latest
fi

print_success "Восстановление завершено"
echo ""
echo "📊 Проверьте статус: $PROJECT_ROOT/scripts/07-manage.sh status"