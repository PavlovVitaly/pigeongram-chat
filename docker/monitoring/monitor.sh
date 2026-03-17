#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Конфигурация
COMPOSE_FILE="docker-compose.yml"
PROJECT_NAME="pigeongram"
GRAFANA_URL="http://localhost:3000"
PROMETHEUS_URL="http://localhost:9090"
MINIO_CONTAINER="pigeongram_minio"

# Функции для вывода
print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   PigeonGram Monitoring Manager v1.0${NC}"
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

# Проверка наличия docker-compose
check_docker() {
    if ! command -v docker-compose &> /dev/null; then
        print_error "docker-compose не найден"
        exit 1
    fi
}

# Проверка существования compose файла
check_compose() {
    if [ ! -f "$COMPOSE_FILE" ]; then
        print_error "Файл $COMPOSE_FILE не найден в текущей директории"
        exit 1
    fi
}

# Проверка статуса контейнеров
check_status() {
    local service=$1
    if [ -z "$service" ]; then
        docker-compose ps
    else
        docker-compose ps | grep "$service"
    fi
}

# Ожидание готовности сервиса
wait_for_service() {
    local service=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    print_info "Ожидание готовности $service..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s -o /dev/null -w "%{http_code}" "$url" | grep -q "200\|302\|401"; then
            print_success "$service готов (попытка $attempt)"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    print_error "$service не отвечает после $max_attempts попыток"
    return 1
}

# Обновление токена MinIO
update_minio_token() {
    print_step "Обновление токена MinIO"
    
    if ! docker ps | grep -q "$MINIO_CONTAINER"; then
        print_error "MinIO контейнер не запущен"
        return 1
    fi
    
    if [ -f "minio-token" ]; then
        rm minio-token
    fi
    
    # Получаем новый токен через token-updater
    docker-compose exec -T token-updater sh -c "/scripts/update-minio-token.sh"
    
    if [ $? -eq 0 ]; then
        print_success "Токен MinIO обновлен"
    else
        print_error "Ошибка обновления токена"
    fi
}

# Показать статистику
show_stats() {
    print_step "Статистика мониторинга"
    
    # Размер данных Prometheus
    if docker ps | grep -q "pigeongram_prometheus"; then
        PROM_SIZE=$(docker exec pigeongram_prometheus du -sh /prometheus 2>/dev/null | cut -f1)
        print_info "Prometheus data: ${PROM_SIZE:-unknown}"
    fi
    
    # Размер данных Grafana
    if docker ps | grep -q "pigeongram_grafana"; then
        GRAFANA_SIZE=$(docker run --rm -v pigeongram_grafana_data:/data alpine du -sh /data 2>/dev/null | cut -f1)
        print_info "Grafana data: ${GRAFANA_SIZE:-unknown}"
    fi
    
    # Статистика запросов к метрикам
    echo ""
    docker-compose logs --tail=10 prometheus 2>/dev/null | grep "scrape" | tail -5
}

# Показать логи конкретного сервиса
show_logs() {
    local service=$1
    local lines=${2:-50}
    
    if [ -z "$service" ]; then
        print_error "Укажите сервис (prometheus, grafana, node-exporter, cadvisor, postgres-exporter, redis-exporter, token-updater)"
        return 1
    fi
    
    docker-compose logs --tail=$lines -f "$service"
}

# Запуск мониторинга
start_monitoring() {
    print_step "Запуск мониторинга"
    
    # Проверяем наличие сети
    if ! docker network inspect pigeongram_network >/dev/null 2>&1; then
        print_info "Создание сети pigeongram_network"
        docker network create pigeongram_network
    fi
    
    # Создаем файл для токена если его нет
    if [ ! -f "minio-token" ]; then
        touch minio-token
        chmod 666 minio-token
        print_info "Создан файл minio-token"
    fi
    
    # Запускаем контейнеры
    docker-compose up -d
    print_success "Контейнеры запущены"
    
    # Ждем готовности сервисов
    wait_for_service "Prometheus" "http://localhost:9090/-/healthy"
    wait_for_service "Grafana" "http://localhost:3000/api/health"
    
    # Показываем статус
    docker-compose ps
}

# Остановка мониторинга
stop_monitoring() {
    print_step "Остановка мониторинга"
    docker-compose down
    print_success "Мониторинг остановлен"
}

# Перезапуск мониторинга
restart_monitoring() {
    print_step "Перезапуск мониторинга"
    docker-compose restart
    print_success "Мониторинг перезапущен"
    
    # Ждем готовности
    wait_for_service "Prometheus" "http://localhost:9090/-/healthy"
}

# Очистка данных
clean_monitoring() {
    print_step "Очистка данных мониторинга"
    print_warning "Это удалит все собранные метрики и данные!"
    read -p "Продолжить? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker-compose down -v
        rm -f minio-token
        print_success "Данные мониторинга очищены"
    else
        print_info "Очистка отменена"
    fi
}

# Показать дашборды
show_dashboards() {
    print_step "Доступные дашборды"
    echo ""
    echo "📊 Предустановленные дашборды:"
    echo "   1. PigeonGram Main Dashboard - основной дашборд"
    echo "   2. Node Exporter Full - метрики сервера"
    echo "   3. Docker Monitoring - метрики контейнеров"
    echo "   4. PostgreSQL - метрики БД"
    echo "   5. Redis - метрики кэша"
    echo ""
    
    # Проверяем, загружены ли дашборды
    if docker exec pigeongram_grafana ls /etc/grafana/provisioning/dashboards 2>/dev/null; then
        print_success "Дашборды сконфигурированы"
    else
        print_warning "Дашборды не настроены автоматически"
    fi
}

# Проверка здоровья
health_check() {
    print_step "Проверка здоровья мониторинга"
    
    local all_ok=true
    
    # Проверка Prometheus
    if curl -s "http://localhost:9090/-/healthy" >/dev/null; then
        print_success "Prometheus: OK"
        
        # Проверка targets
        local targets=$(curl -s "http://localhost:9090/api/v1/targets" | grep -c '"health":"up"')
        print_info "   Активных targets: $targets"
    else
        print_error "Prometheus: недоступен"
        all_ok=false
    fi
    
    # Проверка Grafana
    if curl -s "http://localhost:3000/api/health" | grep -q "ok"; then
        print_success "Grafana: OK"
    else
        print_error "Grafana: недоступна"
        all_ok=false
    fi
    
    # Проверка exporters
    local exporters=(
        "node-exporter:9100"
        "cadvisor:8080"
        "postgres-exporter:9187"
        "redis-exporter:9121"
    )
    
    for exporter in "${exporters[@]}"; do
        name="${exporter%:*}"
        port="${exporter#*:}"
        if docker exec pigeongram_prometheus wget -q -O- "http://$name:$port/metrics" >/dev/null 2>&1; then
            print_success "$name: OK"
        else
            print_warning "$name: недоступен внутри сети"
            all_ok=false
        fi
    done
    
    # Проверка MinIO токена
    if [ -f "minio-token" ] && [ -s "minio-token" ]; then
        print_success "MinIO токен: OK"
    else
        print_warning "MinIO токен: отсутствует или пуст"
    fi
    
    if $all_ok; then
        print_success "Мониторинг работает нормально"
    else
        print_warning "Есть проблемы с мониторингом"
    fi
}

# Интерактивное меню
show_menu() {
    clear
    print_header
    echo ""
    echo "  Доступные команды:"
    echo ""
    echo "  ${CYAN}start${NC}    - Запустить мониторинг"
    echo "  ${CYAN}stop${NC}     - Остановить мониторинг"
    echo "  ${CYAN}restart${NC}  - Перезапустить мониторинг"
    echo "  ${CYAN}status${NC}   - Показать статус контейнеров"
    echo "  ${CYAN}logs${NC}     - Показать логи (использование: $0 logs prometheus)"
    echo "  ${CYAN}stats${NC}    - Показать статистику"
    echo "  ${CYAN}health${NC}   - Проверить здоровье системы"
    echo "  ${CYAN}token${NC}    - Обновить токен MinIO"
    echo "  ${CYAN}dashboards${NC} - Показать дашборды"
    echo "  ${CYAN}clean${NC}    - Очистить все данные (с подтверждением)"
    echo "  ${CYAN}menu${NC}     - Показать это меню"
    echo "  ${CYAN}help${NC}     - Показать справку"
    echo ""
}

# Справка
show_help() {
    print_header
    echo ""
    echo "  Использование: $0 [КОМАНДА] [АРГУМЕНТЫ]"
    echo ""
    echo "  Команды:"
    echo "    start                 - Запустить мониторинг"
    echo "    stop                  - Остановить мониторинг"
    echo "    restart               - Перезапустить мониторинг"
    echo "    status [сервис]       - Показать статус"
    echo "    logs [сервис] [строк] - Показать логи"
    echo "    stats                 - Показать статистику"
    echo "    health                - Проверить здоровье"
    echo "    token                 - Обновить токен MinIO"
    echo "    dashboards            - Показать дашборды"
    echo "    clean                 - Очистить данные"
    echo "    menu                  - Показать меню"
    echo "    help                  - Показать эту справку"
    echo ""
    echo "  Сервисы: prometheus, grafana, node-exporter, cadvisor,"
    echo "           postgres-exporter, redis-exporter, token-updater"
    echo ""
    echo "  Примеры:"
    echo "    $0 start              # Запустить мониторинг"
    echo "    $0 logs prometheus 50 # Показать 50 строк логов Prometheus"
    echo "    $0 status             # Показать статус всех сервисов"
    echo "    $0 token              # Обновить токен MinIO"
    echo ""
}

# Основная логика
main() {
    check_docker
    check_compose
    
    case "${1:-menu}" in
        start)
            start_monitoring
            ;;
        stop)
            stop_monitoring
            ;;
        restart)
            restart_monitoring
            ;;
        status)
            check_status "$2"
            ;;
        logs)
            show_logs "$2" "${3:-50}"
            ;;
        stats)
            show_stats
            ;;
        health)
            health_check
            ;;
        token)
            update_minio_token
            ;;
        dashboards)
            show_dashboards
            ;;
        clean)
            clean_monitoring
            ;;
        menu)
            show_menu
            read -p "Выберите команду: " cmd
            if [ -n "$cmd" ]; then
                main "$cmd"
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