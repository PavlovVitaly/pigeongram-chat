#!/bin/bash

# =====================================================
# PigeonGram Universal Management Script
# =====================================================

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Конфигурация
PROJECT_ROOT="/opt/pigeongram"
POSTGRES_DIR="$PROJECT_ROOT/repo/docker/postgres"
MONITORING_DIR="$PROJECT_ROOT/repo/docker/monitoring"
BACKUP_DIR="$PROJECT_ROOT/backups"
LOG_DIR="$PROJECT_ROOT/logs"

# Создание директорий
mkdir -p $BACKUP_DIR $LOG_DIR

# Функции для вывода
print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   PigeonGram Universal Manager v2.0${NC}"
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

# Проверка наличия необходимых директорий
check_dirs() {
    if [ ! -d "$POSTGRES_DIR" ]; then
        print_error "Директория PostgreSQL не найдена: $POSTGRES_DIR"
        exit 1
    fi
    
    if [ ! -d "$MONITORING_DIR" ]; then
        print_warning "Директория мониторинга не найдена: $MONITORING_DIR"
    fi
}

# Проверка статуса всех сервисов
check_all_status() {
    print_step "Статус всех сервисов"
    
    echo ""
    echo "${CYAN}📊 PostgreSQL:${NC}"
    if [ -f "$POSTGRES_DIR/manage.sh" ]; then
        cd $POSTGRES_DIR
        ./manage.sh status
    else
        print_error "manage.sh не найден в $POSTGRES_DIR"
    fi
    
    echo ""
    echo "${CYAN}📊 Monitoring:${NC}"
    if [ -f "$MONITORING_DIR/monitor.sh" ]; then
        cd $MONITORING_DIR
        ./monitor.sh status
    else
        docker-compose -f $MONITORING_DIR/docker-compose.yml ps 2>/dev/null || print_warning "Мониторинг не настроен"
    fi
    
    echo ""
    echo "${CYAN}📊 Application:${NC}"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "pigeongram_app" || echo "   Приложение не запущено"
}

# Запуск всех сервисов
start_all() {
    print_step "Запуск всех сервисов"
    
    # Запускаем PostgreSQL
    print_info "Запуск PostgreSQL..."
    cd $POSTGRES_DIR
    ./manage.sh start
    
    # Запускаем мониторинг если есть
    if [ -f "$MONITORING_DIR/monitor.sh" ]; then
        print_info "Запуск мониторинга..."
        cd $MONITORING_DIR
        ./monitor.sh start
    fi
    
    # Запускаем приложение
    print_info "Запуск приложения..."
    cd $PROJECT_ROOT/repo
    if ! docker ps | grep -q pigeongram_app; then
        docker run -d \
            --name pigeongram_app \
            --restart unless-stopped \
            -p 8080:8080 \
            --network pigeongram_network \
            -v $LOG_DIR:/app/logs \
            --env-file $PROJECT_ROOT/config/.env.production \
            pigeongram:latest
        print_success "Приложение запущено"
    else
        print_warning "Приложение уже запущено"
    fi
    
    print_success "Все сервисы запущены"
    check_all_status
}

# Остановка всех сервисов
stop_all() {
    print_step "Остановка всех сервисов"
    
    # Останавливаем приложение
    print_info "Остановка приложения..."
    docker stop pigeongram_app 2>/dev/null && docker rm pigeongram_app 2>/dev/null
    
    # Останавливаем мониторинг
    if [ -f "$MONITORING_DIR/monitor.sh" ]; then
        print_info "Остановка мониторинга..."
        cd $MONITORING_DIR
        ./monitor.sh stop
    fi
    
    # Останавливаем PostgreSQL
    print_info "Остановка PostgreSQL..."
    cd $POSTGRES_DIR
    ./manage.sh stop
    
    print_success "Все сервисы остановлены"
}

# Перезапуск всех сервисов
restart_all() {
    print_step "Перезапуск всех сервисов"
    stop_all
    sleep 5
    start_all
}

# Создание бэкапа всех данных
backup_all() {
    local DATE=$(date +%Y%m%d_%H%M%S)
    local BACKUP_FILE="$BACKUP_DIR/full_backup_$DATE"
    
    print_step "Создание полного бэкапа"
    
    # Бэкап PostgreSQL
    print_info "Бэкап PostgreSQL..."
    cd $POSTGRES_DIR
    ./manage.sh backup
    cp backups/*.sql "$BACKUP_FILE-postgres.sql" 2>/dev/null || true
    
    # Бэкап мониторинга
    if docker volume ls | grep -q pigeongram_prometheus_data; then
        print_info "Бэкап метрик Prometheus..."
        docker run --rm -v pigeongram_prometheus_data:/data -v $BACKUP_DIR:/backup alpine \
            tar -czf "/backup/prometheus_$DATE.tar.gz" -C /data . 2>/dev/null || true
    fi
    
    # Бэкап конфигурации
    print_info "Бэкап конфигурации..."
    tar -czf "$BACKUP_FILE-config.tar.gz" -C $PROJECT_ROOT config/ .env* 2>/dev/null || true
    
    # Бэкап логов
    if [ -d "$LOG_DIR" ] && [ "$(ls -A $LOG_DIR)" ]; then
        print_info "Бэкап логов..."
        tar -czf "$BACKUP_FILE-logs.tar.gz" -C $PROJECT_ROOT logs/ 2>/dev/null || true
    fi
    
    print_success "Бэкап завершен: $BACKUP_FILE"
    ls -lh $BACKUP_FILE*
}

# Восстановление из бэкапа
restore_all() {
    if [ -z "$1" ]; then
        print_error "Укажите дату бэкапа (например: 20250317_143022)"
        echo ""
        echo "Доступные бэкапы:"
        ls $BACKUP_DIR/full_backup_*-postgres.sql 2>/dev/null | sed 's/.*full_backup_\(.*\)-postgres.sql/\1/'
        exit 1
    fi
    
    local DATE=$1
    
    print_step "Восстановление из бэкапа $DATE"
    print_warning "ВНИМАНИЕ: Все текущие данные будут потеряны!"
    read -p "Продолжить? (y/N) " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Восстановление отменено"
        exit 0
    fi
    
    # Останавливаем все сервисы
    stop_all
    
    # Восстанавливаем PostgreSQL
    if [ -f "$BACKUP_DIR/full_backup_$DATE-postgres.sql" ]; then
        print_info "Восстановление PostgreSQL..."
        cd $POSTGRES_DIR
        mkdir -p backups
        cp "$BACKUP_DIR/full_backup_$DATE-postgres.sql" backups/
        ./manage.sh restore "backups/full_backup_$DATE-postgres.sql"
    fi
    
    # Восстанавливаем конфигурацию
    if [ -f "$BACKUP_DIR/full_backup_$DATE-config.tar.gz" ]; then
        print_info "Восстановление конфигурации..."
        tar -xzf "$BACKUP_DIR/full_backup_$DATE-config.tar.gz" -C $PROJECT_ROOT
    fi
    
    # Восстанавливаем логи
    if [ -f "$BACKUP_DIR/full_backup_$DATE-logs.tar.gz" ]; then
        print_info "Восстановление логов..."
        tar -xzf "$BACKUP_DIR/full_backup_$DATE-logs.tar.gz" -C $PROJECT_ROOT
    fi
    
    # Запускаем сервисы
    start_all
    
    print_success "Восстановление завершено"
}

# Обновление приложения
update_app() {
    print_step "Обновление приложения"
    
    # Загружаем конфигурацию
    if [ -f "$PROJECT_ROOT/config/.env.production" ]; then
        source "$PROJECT_ROOT/config/.env.production"
    fi
    
    GIT_REPO="${GITHUB_URL:-https://github.com/PavlovVitaly/pigeongram.git}"
    
    print_info "Репозиторий: https://github.com/${GITHUB_USER}/${GITHUB_REPO}"
    
    # Создаем бэкап перед обновлением
    backup_all
    
    # Обновляем код
    cd $PROJECT_ROOT/repo
    git remote set-url origin $GIT_REPO
    git pull origin main
    
    # Пересобираем приложение
    docker stop pigeongram_app 2>/dev/null
    docker rm pigeongram_app 2>/dev/null
    docker build -t pigeongram:latest .
    
    # Запускаем новую версию
    docker run -d \
        --name pigeongram_app \
        --restart unless-stopped \
        -p 8080:8080 \
        --network pigeongram_network \
        -v $LOG_DIR:/app/logs \
        --env-file $PROJECT_ROOT/config/.env.production \
        pigeongram:latest
    
    print_success "Приложение обновлено"
}

# Показать логи
show_logs() {
    local service=$1
    local lines=${2:-100}
    
    case $service in
        app)
            docker logs --tail=$lines -f pigeongram_app
            ;;
        postgres)
            cd $POSTGRES_DIR && ./manage.sh logs
            ;;
        monitoring)
            if [ -f "$MONITORING_DIR/monitor.sh" ]; then
                cd $MONITORING_DIR && ./monitor.sh logs
            else
                docker-compose -f $MONITORING_DIR/docker-compose.yml logs --tail=$lines
            fi
            ;;
        all)
            print_info "Логи приложения:"
            docker logs --tail=50 pigeongram_app 2>/dev/null || echo "   Приложение не запущено"
            echo ""
            print_info "Логи PostgreSQL:"
            cd $POSTGRES_DIR && ./manage.sh logs | tail -20
            echo ""
            if [ -d "$MONITORING_DIR" ]; then
                print_info "Логи мониторинга:"
                docker-compose -f $MONITORING_DIR/docker-compose.yml logs --tail=20 2>/dev/null || echo "   Мониторинг не запущен"
            fi
            ;;
        *)
            print_error "Неизвестный сервис: $service"
            echo "Доступные сервисы: app, postgres, monitoring, all"
            ;;
    esac
}

# Показать метрики
show_metrics() {
    print_step "Метрики системы"
    
    # Метрики приложения
    echo "${CYAN}📊 Application Metrics:${NC}"
    curl -s http://localhost:8080/metrics | grep -E "pigeongram_(connections|messages|files|total)" | head -10 || echo "   Метрики недоступны"
    
    # Метрики PostgreSQL через manage.sh
    echo ""
    echo "${CYAN}📊 PostgreSQL Metrics:${NC}"
    cd $POSTGRES_DIR && ./manage.sh stats 2>/dev/null || echo "   Статистика PostgreSQL недоступна"
    
    # Метрики через мониторинг
    if [ -f "$MONITORING_DIR/monitor.sh" ]; then
        echo ""
        echo "${CYAN}📊 Monitoring Stats:${NC}"
        cd $MONITORING_DIR && ./monitor.sh stats 2>/dev/null || echo "   Статистика мониторинга недоступна"
    fi
}

# Интерактивное меню
show_menu() {
    clear
    print_header
    echo ""
    echo "  Доступные команды:"
    echo ""
    echo "  ${CYAN}status${NC}      - Показать статус всех сервисов"
    echo "  ${CYAN}start${NC}       - Запустить все сервисы"
    echo "  ${CYAN}stop${NC}        - Остановить все сервисы"
    echo "  ${CYAN}restart${NC}     - Перезапустить все сервисы"
    echo "  ${CYAN}backup${NC}      - Создать полный бэкап"
    echo "  ${CYAN}restore [date]${NC} - Восстановить из бэкапа"
    echo "  ${CYAN}update${NC}      - Обновить приложение"
    echo "  ${CYAN}logs [service]${NC} - Показать логи"
    echo "  ${CYAN}metrics${NC}     - Показать метрики"
    echo "  ${CYAN}postgres${NC}    - Управление PostgreSQL (через manage.sh)"
    echo "  ${CYAN}monitor${NC}     - Управление мониторингом (через monitor.sh)"
    echo "  ${CYAN}menu${NC}        - Показать это меню"
    echo "  ${CYAN}help${NC}        - Показать справку"
    echo ""
    echo "  Примеры:"
    echo "    $0 status"
    echo "    $0 logs app 50"
    echo "    $0 postgres backup"
    echo "    $0 monitor health"
    echo ""
}

# Справка
show_help() {
    print_header
    echo ""
    echo "  Использование: $0 [КОМАНДА] [АРГУМЕНТЫ]"
    echo ""
    echo "  Основные команды:"
    echo "    status                - Статус всех сервисов"
    echo "    start                 - Запустить все"
    echo "    stop                  - Остановить все"
    echo "    restart               - Перезапустить все"
    echo "    backup                - Полный бэкап"
    echo "    restore [date]        - Восстановление"
    echo "    update                - Обновление приложения"
    echo "    logs [service] [n]    - Логи (app/postgres/monitoring/all)"
    echo "    metrics               - Метрики системы"
    echo ""
    echo "  Команды PostgreSQL (через manage.sh):"
    echo "    postgres start|stop|restart|backup|restore|status|logs"
    echo ""
    echo "  Команды мониторинга (через monitor.sh):"
    echo "    monitor start|stop|status|health|token|dashboards"
    echo ""
}

# Основная логика
main() {
    check_dirs
    
    case "${1:-menu}" in
        status)
            check_all_status
            ;;
        start)
            start_all
            ;;
        stop)
            stop_all
            ;;
        restart)
            restart_all
            ;;
        backup)
            backup_all
            ;;
        restore)
            restore_all "$2"
            ;;
        update)
            update_app
            ;;
        logs)
            show_logs "$2" "${3:-100}"
            ;;
        metrics)
            show_metrics
            ;;
        postgres)
            shift
            if [ -f "$POSTGRES_DIR/manage.sh" ]; then
                cd $POSTGRES_DIR
                ./manage.sh "$@"
            else
                print_error "manage.sh не найден"
            fi
            ;;
        monitor)
            shift
            if [ -f "$MONITORING_DIR/monitor.sh" ]; then
                cd $MONITORING_DIR
                ./monitor.sh "$@"
            else
                cd $MONITORING_DIR
                docker-compose "$@"
            fi
            ;;
        menu)
            show_menu
            read -p "Выберите команду: " cmd
            if [ -n "$cmd" ]; then
                main $cmd
            fi
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Неизвестная команда: $1"
            show_help
            exit 1
            ;;
    esac
}

# Запуск
main "$@"