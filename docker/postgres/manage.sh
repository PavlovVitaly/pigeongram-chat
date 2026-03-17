#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

COMPOSE_FILE="docker-compose.yml"
POSTGRES_VOLUME="pigeongram_postgres_data"
REDIS_VOLUME="pigeongram_redis_data"
MINIO_VOLUME="pigeongram_minio_data"

print_header() {
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   PigeonGram PostgreSQL Manager${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_step() {
    echo -e "\n${YELLOW}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Функция проверки и создания сети
ensure_network() {
    local network_name="pigeongram_network"
    
    if ! docker network inspect $network_name >/dev/null 2>&1; then
        echo "🌐 Создание сети $network_name..."
        docker network create $network_name
        print_success "Сеть создана"
    else
        print_success "Сеть $network_name уже существует"
    fi
}

# Функция синхронизации пароля с .env
sync_password() {
    local env_file="${1:-/opt/pigeongram/config/.env.production}"
    
    print_step "Синхронизация пароля PostgreSQL"
    
    if [ ! -f "$env_file" ]; then
        print_error "Файл конфигурации не найден: $env_file"
        return 1
    fi
    
    local db_password=$(grep DB_PASSWORD "$env_file" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$db_password" ]; then
        print_error "Не удалось получить пароль из $env_file"
        return 1
    fi
    
    echo -e "${YELLOW}🔄 Синхронизация пароля для пользователя pigeongram...${NC}"
    
    # Проверяем, запущен ли контейнер
    if ! docker ps | grep -q pigeongram_postgres; then
        print_error "Контейнер PostgreSQL не запущен"
        return 1
    fi
    
    # Пробуем установить пароль разными способами
    docker exec -i pigeongram_postgres psql -U postgres -c "ALTER USER pigeongram WITH PASSWORD '$db_password';" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        print_success "Пароль успешно синхронизирован (через postgres)"
    else
        # Пробуем через пользователя pigeongram
        docker exec -i pigeongram_postgres psql -U pigeongram -d postgres -c "ALTER USER pigeongram WITH PASSWORD '$db_password';" 2>/dev/null
        
        if [ $? -eq 0 ]; then
            print_success "Пароль успешно синхронизирован (через pigeongram)"
        else
            print_warning "Не удалось синхронизировать пароль автоматически"
            return 1
        fi
    fi
    
    # Проверяем подключение
    if docker exec -i pigeongram_postgres psql -U pigeongram -d pigeongram -c "SELECT 1;" 2>/dev/null; then
        print_success "✅ Подключение к PostgreSQL работает"
    else
        print_error "❌ Не удалось подключиться к PostgreSQL"
        return 1
    fi
}

start() {
    print_step "Запуск контейнеров"
    ensure_network
    docker-compose up -d
    print_success "Контейнеры запущены"
    show_status
    
    # Синхронизируем пароль после запуска
    sync_password
}

stop() {
    print_step "Остановка контейнеров"
    docker-compose down
    print_success "Контейнеры остановлены"
}

restart() {
    print_step "Перезапуск контейнеров"
    docker-compose restart
    print_success "Контейнеры перезапущены"
    show_status
    sync_password
}

status() {
    print_step "Статус контейнеров"
    docker-compose ps
}

logs() {
    print_step "Логи PostgreSQL"
    docker-compose logs --tail=50 -f postgres
}

backup() {
    print_step "Создание бэкапа"
    local DATE=$(date +%Y%m%d_%H%M%S)
    mkdir -p backups
    docker exec pigeongram_postgres pg_dump -U pigeongram pigeongram > "backups/backup_$DATE.sql"
    print_success "Бэкап создан: backups/backup_$DATE.sql"
}

restore() {
    if [ -z "$1" ]; then
        print_error "Укажите файл для восстановления"
        echo "Использование: $0 restore backups/backup.sql"
        return 1
    fi
    
    print_step "Восстановление из бэкапа $1"
    cat "$1" | docker exec -i pigeongram_postgres psql -U pigeongram -d pigeongram
    print_success "Восстановление завершено"
}

connect() {
    print_step "Подключение к PostgreSQL"
    docker exec -it pigeongram_postgres psql -U pigeongram -d pigeongram
}

clean() {
    print_step "Остановка и удаление контейнеров"
    docker-compose down
    print_success "Контейнеры остановлены и удалены"
}

show_status() {
    echo ""
    echo "📊 Статус контейнеров:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "pigeongram_postgres|pigeongram_redis|pigeongram_minio" || echo "   Контейнеры не запущены"
}

show_help() {
    print_header
    echo ""
    echo "  Использование: $0 [КОМАНДА] [АРГУМЕНТЫ]"
    echo ""
    echo "  Команды:"
    echo "    start             - Запустить контейнеры"
    echo "    stop              - Остановить контейнеры"
    echo "    restart           - Перезапустить контейнеры"
    echo "    status            - Показать статус"
    echo "    logs              - Показать логи PostgreSQL"
    echo "    backup            - Создать бэкап БД"
    echo "    restore FILE      - Восстановить из бэкапа"
    echo "    connect           - Подключиться к PostgreSQL"
    echo "    clean             - Остановить и удалить контейнеры"
    echo "    sync-pass [FILE]  - Синхронизировать пароль с .env файлом"
    echo "    help              - Показать эту справку"
    echo ""
    echo "  Примеры:"
    echo "    $0 start"
    echo "    $0 sync-pass /opt/pigeongram/config/.env.production"
    echo ""
}

case "${1:-help}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    backup)
        backup
        ;;
    restore)
        restore "$2"
        ;;
    connect)
        connect
        ;;
    clean)
        clean
        ;;
    sync-pass)
        sync_password "$2"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Неизвестная команда: $1"
        show_help
        exit 1
        ;;
esac