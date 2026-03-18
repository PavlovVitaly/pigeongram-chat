#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
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

# 👇 ИСПРАВЛЕННАЯ функция синхронизации пароля Redis
sync_redis_password() {
    local env_file="${1:-/opt/pigeongram/config/.env.production}"
    
    print_step "Синхронизация пароля Redis"
    
    if [ ! -f "$env_file" ]; then
        print_error "Файл конфигурации не найден: $env_file"
        return 1
    fi
    
    local redis_password=$(grep REDIS_PASSWORD "$env_file" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$redis_password" ]; then
        print_error "REDIS_PASSWORD не найден в $env_file"
        return 1
    fi
    
    # Проверяем, запущен ли контейнер
    if ! docker ps | grep -q pigeongram_redis; then
        print_error "Контейнер Redis не запущен"
        return 1
    fi
    
    # Шаг 1: Проверяем, может уже работает с правильным паролем
    if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
        print_success "✅ Redis уже работает с правильным паролем"
        return 0
    fi
    
    print_warning "⚠️ Пароль Redis не совпадает. Пробуем восстановить..."
    
    # Шаг 2: Пробуем подключиться без пароля (если пароль еще не установлен)
    if docker exec pigeongram_redis redis-cli PING 2>/dev/null | grep -q "PONG"; then
        print_info "Redis работает без пароля, устанавливаем..."
        docker exec pigeongram_redis redis-cli CONFIG SET requirepass "$redis_password"
        
        if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
            print_success "✅ Пароль Redis успешно установлен"
            return 0
        fi
    fi
    
    # Шаг 3: Пробуем аутентифицироваться с неправильным паролем (чтобы понять текущий статус)
    if docker exec pigeongram_redis redis-cli AUTH wrongpass 2>&1 | grep -q "ERR AUTH"; then
        # Значит пароль уже установлен, но другой
        print_warning "⚠️ В Redis уже установлен другой пароль"
        
        # Шаг 4: Перезапускаем Redis без пароля
        print_info "Перезапускаем Redis без пароля для перенастройки..."
        
        # Останавливаем Redis
        docker-compose stop redis
        docker-compose rm -f redis
        
        # Временно убираем пароль из команды запуска
        # Создаем временный docker-compose без пароля
        sed -i 's/--requirepass ${REDIS_PASSWORD:-redis_secret}//' docker-compose.yml
        docker-compose up -d redis
        sleep 5
        
        # Устанавливаем новый пароль
        docker exec pigeongram_redis redis-cli CONFIG SET requirepass "$redis_password"
        
        # Возвращаем оригинальный docker-compose.yml
        git checkout docker-compose.yml 2>/dev/null || true
        
        # Проверяем
        if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
            print_success "✅ Пароль Redis успешно установлен"
        else
            print_error "❌ Не удалось установить пароль Redis"
            return 1
        fi
    else
        # Непонятная ситуация, пробуем простой CONFIG SET
        docker exec pigeongram_redis redis-cli CONFIG SET requirepass "$redis_password" 2>/dev/null
        
        if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
            print_success "✅ Пароль Redis установлен"
        else
            print_error "❌ Критическая ошибка Redis"
            return 1
        fi
    fi
}

# 👇 ИСПРАВЛЕННАЯ функция синхронизации пароля MinIO
sync_minio_password() {
    local env_file="${1:-/opt/pigeongram/config/.env.production}"
    
    print_step "Синхронизация пароля MinIO"
    
    if [ ! -f "$env_file" ]; then
        print_error "Файл конфигурации не найден: $env_file"
        return 1
    fi
    
    local minio_password=$(grep MINIO_SECRET_KEY "$env_file" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$minio_password" ]; then
        print_error "MINIO_SECRET_KEY не найден в $env_file"
        return 1
    fi
    
    # Проверяем, запущен ли контейнер
    if ! docker ps | grep -q pigeongram_minio; then
        print_error "Контейнер MinIO не запущен"
        return 1
    fi
    
    # Проверяем, отвечает ли MinIO
    if ! curl -s http://localhost:9000/minio/health/live >/dev/null; then
        print_error "MinIO не отвечает на запросы"
        return 1
    fi
    
    # Пробуем настроить алиас с паролем из .env
    if docker exec pigeongram_minio mc alias set myminio http://localhost:9000 minioadmin "$minio_password" 2>/dev/null; then
        print_success "✅ Пароль MinIO синхронизирован"
    else
        print_warning "⚠️ MinIO запущен, но пароль может отличаться от .env"
        print_info "   Текущий пароль MinIO: minioadmin (по умолчанию)"
        print_info "   Для смены пароля выполните:"
        echo "   docker exec -it pigeongram_minio mc admin user svcacct add --access-key minioadmin --secret-key \"$minio_password\" myminio"
    fi
}

# Существующая функция синхронизации пароля PostgreSQL
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

# Функция проверки всех контейнеров
check_all_containers() {
    print_step "Проверка всех контейнеров"
    
    echo "📊 PostgreSQL:"
    docker ps -a --filter "name=pigeongram_postgres" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "   Не найден"
    
    echo ""
    echo "📊 Redis:"
    docker ps -a --filter "name=pigeongram_redis" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "   Не найден"
    
    echo ""
    echo "📊 MinIO:"
    docker ps -a --filter "name=pigeongram_minio" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "   Не найден"
    
    echo ""
    echo "📊 Redis Commander:"
    docker ps -a --filter "name=pigeongram_redis_commander" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "   Не найден"
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
    print_step "Запуск контейнеров"
    ensure_network
    
    # Запускаем все контейнеры
    docker-compose up -d
    
    print_success "Контейнеры запущены"
    sleep 5
    
    # Проверяем каждый сервис
    check_service "postgres"
    check_service "redis"
    check_service "minio"
    
    show_status
    
    # Синхронизируем пароли всех сервисов
    sync_password
    sync_redis_password
    sync_minio_password
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
    sleep 5
    show_status
    
    # Синхронизируем пароли всех сервисов
    sync_password
    sync_redis_password
    sync_minio_password
}

status() {
    print_step "Статус контейнеров"
    docker-compose ps
}

logs() {
    local service=${1:-postgres}
    print_step "Логи $service"
    docker-compose logs --tail=50 -f "$service"
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
    echo "    logs [service]    - Показать логи (postgres/redis/minio)"
    echo "    backup            - Создать бэкап БД"
    echo "    restore FILE      - Восстановить из бэкапа"
    echo "    connect           - Подключиться к PostgreSQL"
    echo "    clean             - Остановить и удалить контейнеры"
    echo "    sync-pass [FILE]  - Синхронизировать пароль PostgreSQL"
    echo "    sync-redis [FILE] - Синхронизировать пароль Redis"
    echo "    sync-minio [FILE] - Синхронизировать пароль MinIO"
    echo "    check-all         - Проверить все контейнеры"
    echo "    help              - Показать эту справку"
    echo ""
    echo "  Примеры:"
    echo "    $0 start"
    echo "    $0 logs minio"
    echo "    $0 check-all"
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
    sync-pass)
        sync_password "$2"
        ;;
    sync-redis)
        sync_redis_password "$2"
        ;;
    sync-minio)
        sync_minio_password "$2"
        ;;
    check-all)
        check_all_containers
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