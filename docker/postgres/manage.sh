#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Конфигурация
COMPOSE_FILE="docker-compose.local.yml"
POSTGRES_VOLUME="pigeongram_postgres_data"
REDIS_VOLUME="pigeongram_redis_data"
MINIO_VOLUME="pigeongram_minio_data"

print_header() {
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   PigeonGram Local Development Manager${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_step() {
    echo -e "\n${CYAN}▶ $1${NC}"
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

# Функция проверки локальной конфигурации Nginx
check_nginx_config() {
    if [ -f "../nginx/nginx.local.conf" ]; then
        print_info "Найдена локальная конфигурация Nginx"
        return 0
    else
        print_warning "Локальная конфигурация Nginx не найдена"
        return 1
    fi
}

# Функция проверки конкретного сервиса
check_service() {
    local service=$1
    if docker ps | grep -q "pigeongram_$service"; then
        print_success "$service запущен"
        return 0
    else
        print_warning "$service не запущен"
        return 1
    fi
}

start() {
    print_step "Запуск локальных контейнеров"
    ensure_network
    
    # Проверяем наличие локальной конфигурации Nginx
    check_nginx_config
    
    # Запускаем контейнеры
    docker-compose -f $COMPOSE_FILE up -d
    print_success "Контейнеры запущены"
    show_status
    
    # Показываем информацию о доступе
    show_access_info
}

stop() {
    print_step "Остановка контейнеров"
    docker-compose -f $COMPOSE_FILE down
    print_success "Контейнеры остановлены"
}

restart() {
    print_step "Перезапуск контейнеров"
    docker-compose -f $COMPOSE_FILE restart
    print_success "Контейнеры перезапущены"
    show_status
    show_access_info
}

status() {
    print_step "Статус контейнеров"
    docker-compose -f $COMPOSE_FILE ps
}

logs() {
    local service=${1:-postgres}
    print_step "Логи $service"
    docker-compose -f $COMPOSE_FILE logs --tail=50 -f "$service"
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
    print_step "Полная очистка локальных данных"
    docker-compose -f $COMPOSE_FILE down -v
    print_success "Контейнеры и тома удалены"
}

show_status() {
    echo ""
    echo "📊 Статус локальных контейнеров:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "pigeongram_postgres|pigeongram_redis|pigeongram_minio|pigeongram_nginx" || echo "   Контейнеры не запущены"
}

show_access_info() {
    echo ""
    echo "🌐 Доступные сервисы:"
    echo "   📱 Приложение (через Nginx):  http://localhost"
    echo "   📱 Приложение (прямой доступ): http://localhost:8080"
    echo "   🗄️  MinIO Console:              http://localhost:9001"
    echo "   🔧 pgAdmin:                     http://localhost:5050"
    echo "   🔴 Redis Commander:             http://localhost:8081"
    echo "   📊 MinIO (порт 81):             http://localhost:81"
    
    if [ -f "../nginx/nginx.local.conf" ]; then
        echo ""
        echo "📁 Nginx конфигурация:"
        echo "   📍 Локальная: ../nginx/nginx.local.conf"
    fi
}

show_help() {
    print_header
    echo ""
    echo "  Использование: $0 [КОМАНДА] [АРГУМЕНТЫ]"
    echo ""
    echo "  Команды:"
    echo "    start         - Запустить локальные контейнеры"
    echo "    stop          - Остановить контейнеры"
    echo "    restart       - Перезапустить контейнеры"
    echo "    status        - Показать статус"
    echo "    logs [сервис] - Показать логи (postgres/redis/minio/nginx)"
    echo "    backup        - Создать бэкап БД"
    echo "    restore FILE  - Восстановить из бэкапа"
    echo "    connect       - Подключиться к PostgreSQL"
    echo "    clean         - Полная очистка (с удалением томов)"
    echo "    help          - Показать эту справку"
    echo ""
    echo "  Примеры:"
    echo "    $0 start"
    echo "    $0 logs nginx"
    echo "    $0 clean"
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
        logs "$2"
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
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Неизвестная команда: $1"
        show_help
        exit 1
        ;;
esac