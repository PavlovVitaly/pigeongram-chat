#!/bin/bash

# =====================================================
# PigeonGram - Управление PostgreSQL, Redis, MinIO
# =====================================================

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Определяем окружение
ENV=${1:-local}
shift 2>/dev/null || true
CMD=$1

# Пути к файлам конфигурации
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_ENV="$SCRIPT_DIR/.env.local"                     # 👈 ИЗМЕНЕНО: в той же папке
PROD_ENV="/opt/pigeongram/config/.env.production"

# Выбор конфигурации
case $ENV in
    local)
        ENV_FILE="$LOCAL_ENV"
        COMPOSE_FILE="docker-compose.local.yml"
        ENV_NAME="LOCAL (разработка)"
        ;;
    prod|production)
        ENV_FILE="$PROD_ENV"
        COMPOSE_FILE="docker-compose.yml"
        ENV_NAME="PRODUCTION"
        ;;
    *)
        echo -e "${RED}❌ Неизвестное окружение: $ENV${NC}"
        echo "Использование: $0 {local|prod} {start|stop|status|logs|backup|restore|connect|clean}"
        exit 1
        ;;
esac

# Проверка наличия файла конфигурации
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}❌ Файл конфигурации не найден: $ENV_FILE${NC}"
    if [ "$ENV" = "local" ]; then
        echo "📝 Создайте файл .env.local в директории: $SCRIPT_DIR"
        echo "   Пример содержимого:"
        echo "   DB_HOST=localhost"
        echo "   DB_PORT=5432"
        echo "   DB_USER=pigeongram"
        echo "   DB_PASSWORD=pigeongram_secret"
        echo "   DB_NAME=pigeongram"
    else
        echo "📝 Убедитесь, что файл существует на сервере"
    fi
    exit 1
fi

# Загружаем переменные окружения
export $(grep -v '^#' "$ENV_FILE" | xargs)

print_header() {
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   PigeonGram Manager (${ENV_NAME})${NC}"
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
        print_info "Создание сети $network_name..."
        docker network create $network_name
        print_success "Сеть создана"
    else
        print_success "Сеть $network_name уже существует"
    fi
}

# Функция проверки конкретного сервиса
check_service() {
    local service=$1
    if docker ps | grep -q "pigeongram_$service"; then
        return 0
    fi
    return 1
}

# Функция ожидания сервиса
wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if check_service "$service"; then
            print_success "$service запущен (попытка $attempt)"
            return 0
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    print_error "$service не запустился"
    return 1
}

# Запуск контейнеров
start() {
    print_step "Запуск контейнеров ($ENV_NAME)"
    
    # Проверяем наличие compose файла
    if [ ! -f "$COMPOSE_FILE" ]; then
        print_error "Файл $COMPOSE_FILE не найден"
        exit 1
    fi
    
    ensure_network
    docker-compose -f "$COMPOSE_FILE" up -d
    print_success "Контейнеры запущены"
    
    # Ожидание готовности сервисов
    print_info "Ожидание инициализации сервисов..."
    sleep 10
    wait_for_service "postgres"
    wait_for_service "redis"
    wait_for_service "minio"
    
    show_status
}

# Остановка контейнеров
stop() {
    print_step "Остановка контейнеров ($ENV_NAME)"
    docker-compose -f "$COMPOSE_FILE" down
    print_success "Контейнеры остановлены"
}

# Перезапуск контейнеров
restart() {
    print_step "Перезапуск контейнеров ($ENV_NAME)"
    docker-compose -f "$COMPOSE_FILE" restart
    print_success "Контейнеры перезапущены"
    show_status
}

# Статус контейнеров
status() {
    print_step "Статус контейнеров ($ENV_NAME)"
    docker-compose -f "$COMPOSE_FILE" ps
}

# Логи
logs() {
    local service=${1:-postgres}
    print_step "Логи $service"
    docker-compose -f "$COMPOSE_FILE" logs --tail=50 -f "$service"
}

# Бэкап базы данных
backup() {
    print_step "Создание бэкапа PostgreSQL"
    local DATE=$(date +%Y%m%d_%H%M%S)
    mkdir -p backups
    docker exec pigeongram_postgres pg_dump -U ${DB_USER:-pigeongram} ${DB_NAME:-pigeongram} > "backups/backup_$DATE.sql"
    print_success "Бэкап создан: backups/backup_$DATE.sql"
}

# Восстановление из бэкапа
restore() {
    if [ -z "$1" ]; then
        print_error "Укажите файл для восстановления"
        echo "Использование: $0 $ENV restore backups/backup.sql"
        return 1
    fi
    
    print_step "Восстановление из бэкапа $1"
    cat "$1" | docker exec -i pigeongram_postgres psql -U ${DB_USER:-pigeongram} -d ${DB_NAME:-pigeongram}
    print_success "Восстановление завершено"
}

# Подключение к PostgreSQL
connect() {
    print_step "Подключение к PostgreSQL"
    docker exec -it pigeongram_postgres psql -U ${DB_USER:-pigeongram} -d ${DB_NAME:-pigeongram}
}

# Полная очистка
clean() {
    print_step "Полная очистка ($ENV_NAME)"
    print_warning "ВНИМАНИЕ: Это удалит все данные!"
    read -p "Продолжить? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker-compose -f "$COMPOSE_FILE" down -v
        print_success "Контейнеры и тома удалены"
    else
        print_info "Очистка отменена"
    fi
}

# Показать статус
show_status() {
    echo ""
    echo "📊 Статус контейнеров ($ENV_NAME):"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "pigeongram_postgres|pigeongram_redis|pigeongram_minio" || echo "   Контейнеры не запущены"
    
    echo ""
    echo "🔧 Настройки ($ENV_NAME):"
    echo "   PostgreSQL: ${DB_HOST:-не задан}:${DB_PORT:-5432}"
    echo "   Redis: ${REDIS_HOST:-не задан}:${REDIS_PORT:-6379}"
    echo "   MinIO: ${MINIO_ENDPOINT:-не задан}"
}

# Справка
show_help() {
    print_header
    echo ""
    echo "  Использование: $0 {local|prod} {КОМАНДА} [АРГУМЕНТЫ]"
    echo ""
    echo "  Команды:"
    echo "    start             - Запустить контейнеры"
    echo "    stop              - Остановить контейнеры"
    echo "    restart           - Перезапустить контейнеры"
    echo "    status            - Показать статус"
    echo "    logs [сервис]     - Показать логи (postgres/redis/minio)"
    echo "    backup            - Создать бэкап БД"
    echo "    restore FILE      - Восстановить из бэкапа"
    echo "    connect           - Подключиться к PostgreSQL"
    echo "    clean             - Полная очистка (с удалением томов)"
    echo "    help              - Показать эту справку"
    echo ""
    echo "  Файлы конфигурации:"
    echo "    local: $LOCAL_ENV"
    echo "    prod:  $PROD_ENV"
    echo ""
    echo "  Примеры:"
    echo "    $0 local start      # Запуск для локальной разработки"
    echo "    $0 prod start       # Запуск для продакшена"
    echo "    $0 local logs minio # Логи MinIO в локальном режиме"
    echo "    $0 prod backup      # Бэкап в продакшене"
    echo ""
}

# Основная логика
case "$CMD" in
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
        if [ -z "$CMD" ]; then
            show_help
        else
            echo -e "${RED}Неизвестная команда: $CMD${NC}"
            show_help
            exit 1
        fi
        ;;
esac