#!/bin/bash

# =====================================================
# PigeonGram - Полное развертывание одной командой
# =====================================================

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Конфигурация по умолчанию
GITHUB_USER="PavlovVitaly"
GITHUB_REPO="pigeongram"
APP_USER="app"
APP_DIR="/opt/pigeongram"
DOMAIN=${DOMAIN:-"localhost"}

# Функция поиска файла конфигурации
find_env_file() {
    local found_env=""
    
    # 1. Если задана переменная ENV_FILE, используем её
    if [ -n "$ENV_FILE" ]; then
        if [ -f "$ENV_FILE" ]; then
            echo -e "${GREEN}✅ Использую конфигурацию из переменной ENV_FILE: $ENV_FILE${NC}"
            ENV_PATH="$ENV_FILE"
            return 0
        else
            echo -e "${RED}❌ Указанный ENV_FILE не существует: $ENV_FILE${NC}"
            return 1
        fi
    fi
    
    # 2. Ищем .env.production в текущей директории
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    if [ -f "$SCRIPT_DIR/.env.production" ]; then
        echo -e "${GREEN}✅ Найден .env.production в директории скрипта${NC}"
        ENV_PATH="$SCRIPT_DIR/.env.production"
        return 0
    fi
    
    # 3. Ищем .env в текущей директории
    if [ -f "$SCRIPT_DIR/.env" ]; then
        echo -e "${YELLOW}⚠️ Найден .env (не production), используем его${NC}"
        ENV_PATH="$SCRIPT_DIR/.env"
        return 0
    fi
    
    # 4. Ищем в домашней директории
    if [ -f "$HOME/.pigeongram.env" ]; then
        echo -e "${YELLOW}⚠️ Использую $HOME/.pigeongram.env${NC}"
        ENV_PATH="$HOME/.pigeongram.env"
        return 0
    fi
    
    # 5. Ищем в стандартной директории конфигов
    if [ -f "/etc/pigeongram/env" ]; then
        echo -e "${YELLOW}⚠️ Использую /etc/pigeongram/env${NC}"
        ENV_PATH="/etc/pigeongram/env"
        return 0
    fi
    
    echo -e "${RED}❌ Файл конфигурации не найден!${NC}"
    echo -e "Положите .env.production в одно из мест:"
    echo -e "  - Переменная ENV_FILE=/path/to/.env.production"
    echo -e "  - Текущая директория: $SCRIPT_DIR/.env.production"
    echo -e "  - Текущая директория: $SCRIPT_DIR/.env"
    echo -e "  - Домашняя директория: $HOME/.pigeongram.env"
    echo -e "  - Системная: /etc/pigeongram/env"
    return 1
}

# Формируем URL для клонирования
if [ -n "$GITHUB_TOKEN" ]; then
    GITHUB_URL="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
    echo -e "${GREEN}✅ Использую GitHub токен для аутентификации${NC}"
else
    GITHUB_URL="https://github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
    echo -e "${YELLOW}⚠️ GitHub токен не указан. Если репозиторий приватный, клонирование не удастся.${NC}"
fi

# Функции для вывода
print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   $1${NC}"
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

# Генерация случайных паролей
generate_password() {
    openssl rand -base64 32 | tr -d '/+=' | cut -c1-32
}

generate_jwt_secret() {
    openssl rand -base64 48 | tr -d '/+=' | cut -c1-64
}

# Проверка, что скрипт запущен от root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "Этот скрипт должен запускаться от root (sudo)"
        exit 1
    fi
}

# Функция для отладки паролей
debug_passwords() {
    print_step "Отладка паролей"
    
    echo "Содержимое .env.production:"
    cat "$APP_DIR/config/.env.production" | grep -E "PASSWORD|SECRET" || echo "   Пароли не найдены!"
    
    echo ""
    echo "Проверка переменных в docker-compose:"
    cd "$APP_DIR/repo/docker/postgres"
    docker-compose config | grep -E "PASSWORD|SECRET" | head -10
    
    echo ""
    echo "Проверка запущенных контейнеров:"
    docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "redis|minio"
}

# Функция синхронизации пароля PostgreSQL
sync_postgres_password() {
    print_step "Синхронизация пароля PostgreSQL"
    
    sleep 10
    
    local db_password=$(grep DB_PASSWORD "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$db_password" ]; then
        print_error "Не удалось получить пароль из .env.production"
        return 1
    fi
    
    print_info "Синхронизируем пароль для пользователя pigeongram..."
    
    docker exec -i pigeongram_postgres psql -U postgres -c "ALTER USER pigeongram WITH PASSWORD '$db_password';" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        print_success "Пароль успешно синхронизирован (через postgres)"
    else
        docker exec -i pigeongram_postgres psql -U pigeongram -d postgres -c "ALTER USER pigeongram WITH PASSWORD '$db_password';" 2>/dev/null
        
        if [ $? -eq 0 ]; then
            print_success "Пароль успешно синхронизирован (через pigeongram)"
        else
            print_warning "Не удалось синхронизировать пароль автоматически"
        fi
    fi
    
    if docker exec -i pigeongram_postgres psql -U pigeongram -d pigeongram -c "SELECT 1;" 2>/dev/null; then
        print_success "✅ Подключение к PostgreSQL работает"
    else
        print_error "❌ Не удалось подключиться к PostgreSQL"
    fi
}

# Функция синхронизации Redis
sync_redis_password() {
    print_step "Синхронизация пароля Redis"
    
    local redis_password=$(grep REDIS_PASSWORD "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$redis_password" ]; then
        print_warning "REDIS_PASSWORD не найден, использую значение по умолчанию"
        redis_password="redis_secret"
    fi
    
    print_info "Проверка подключения к Redis..."
    
    if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
        print_success "✅ Redis уже работает с правильным паролем"
    else
        print_warning "⚠️ Пароль Redis не совпадает. Перезапускаем..."
        
        cd "$APP_DIR/repo/docker/postgres"
        docker-compose stop redis
        docker-compose rm -f redis
        docker-compose up -d redis
        
        sleep 5
        
        if docker exec pigeongram_redis redis-cli -a "$redis_password" PING 2>/dev/null | grep -q "PONG"; then
            print_success "✅ Redis успешно перезапущен"
        else
            print_error "❌ Не удалось настроить Redis"
            debug_passwords
        fi
    fi
}

# Функция синхронизации MinIO
sync_minio_password() {
    print_step "Синхронизация пароля MinIO"
    
    local minio_password=$(grep MINIO_SECRET_KEY "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    if [ -z "$minio_password" ]; then
        print_warning "MINIO_SECRET_KEY не найден, использую значение по умолчанию"
        minio_password="minioadmin"
    fi
    
    print_info "Проверка подключения к MinIO..."
    
    if curl -s http://localhost:9000/minio/health/live >/dev/null; then
        print_success "✅ MinIO уже работает"
    else
        print_warning "⚠️ MinIO не отвечает. Перезапускаем..."
        
        cd "$APP_DIR/repo/docker/postgres"
        docker-compose stop minio
        docker-compose rm -f minio
        docker-compose up -d minio
        
        sleep 10
        
        if curl -s http://localhost:9000/minio/health/live >/dev/null; then
            print_success "✅ MinIO успешно перезапущен"
        else
            print_error "❌ Не удалось запустить MinIO"
            docker logs pigeongram_minio --tail 20
        fi
    fi
}

# Функции установки сервера
setup_server() {
    print_step "Установка и настройка сервера"
    
    apt update && apt upgrade -y
    print_success "Система обновлена"
    
    apt install -y \
        curl wget git vim htop net-tools \
        ufw fail2ban unattended-upgrades \
        jq ncdu build-essential software-properties-common \
        apt-transport-https ca-certificates gnupg lsb-release
    print_success "Базовые пакеты установлены"
    
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    print_success "Docker установлен"
    
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
    curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    print_success "Docker Compose ${COMPOSE_VERSION} установлен"
    
    if ! id "$APP_USER" &>/dev/null; then
        useradd -m -s /bin/bash "$APP_USER"
        echo "$APP_USER:$(generate_password)" | chpasswd
        print_success "Пользователь $APP_USER создан"
    fi
    
    usermod -aG docker "$APP_USER"
    print_success "Пользователь добавлен в группу docker"
    
    mkdir -p "$APP_DIR"/{repo,data,logs,backups,ssl,config}
    chown -R "$APP_USER":"$APP_USER" "$APP_DIR"
    print_success "Директории созданы в $APP_DIR"
    
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow 22/tcp comment 'SSH'
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'
    ufw allow 8080/tcp comment 'PigeonGram App'
    ufw allow 9090/tcp comment 'Prometheus'
    ufw allow 3000/tcp comment 'Grafana'
    ufw allow 9000/tcp comment 'MinIO API'
    ufw allow 9001/tcp comment 'MinIO Console'
    echo "y" | ufw enable
    print_success "Файрвол настроен"
    
    cat > /etc/fail2ban/jail.local << EOF
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 3600
EOF
    systemctl restart fail2ban
    systemctl enable fail2ban
    print_success "fail2ban настроен"
    
    if [ ! -f /swapfile ]; then
        fallocate -l 2G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
        print_success "Swap файл создан (2GB)"
    fi
    
    cat >> /etc/sysctl.conf << EOF

# PigeonGram optimizations
net.core.somaxconn = 1024
net.ipv4.tcp_max_syn_backlog = 4096
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
vm.swappiness = 10
EOF
    sysctl -p
    print_success "Параметры ядра оптимизированы"
}

# Функции деплоя приложения
deploy_app() {
    print_step "Деплой приложения"
    
    if ! find_env_file; then
        print_error "Не удалось найти файл конфигурации"
        exit 1
    fi
    
    print_info "Копирование конфигурации из $ENV_PATH"
    cp "$ENV_PATH" "$APP_DIR/config/.env.production"
    chmod 600 "$APP_DIR/config/.env.production"
    chown "$APP_USER":"$APP_USER" "$APP_DIR/config/.env.production"
    print_success "Конфигурация скопирована"
    
    if [ ! -f "$APP_DIR/ssl/cert.pem" ]; then
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "$APP_DIR/ssl/key.pem" \
            -out "$APP_DIR/ssl/cert.pem" \
            -subj "/C=RU/ST=Moscow/L=Moscow/O=PigeonGram/CN=${DOMAIN}"
        chown -R "$APP_USER":"$APP_USER" "$APP_DIR/ssl"
        print_success "SSL сертификаты сгенерированы"
    fi
    
    print_step "Клонирование репозитория"
    cd "$APP_DIR"
    
    if [ -d "repo" ]; then
        if [ -d "repo/.git" ]; then
            print_info "Репозиторий уже существует, обновляем..."
            cd repo
            git pull
        else
            print_warning "Директория repo существует, но это не git-репозиторий"
            print_info "Удаляем и клонируем заново..."
            rm -rf repo
            sudo -u "$APP_USER" git clone "$GITHUB_URL" repo
        fi
    else
        print_info "Клонирование из ${GITHUB_URL}"
        sudo -u "$APP_USER" git clone "$GITHUB_URL" repo
        if [ $? -ne 0 ]; then
            print_error "Ошибка клонирования репозитория. Проверьте токен."
            exit 1
        fi
    fi
    print_success "Репозиторий склонирован"
    
    # Запуск инфраструктуры через обновленный manage.sh
    print_step "Запуск PostgreSQL, Redis и MinIO"
    cd "$APP_DIR/repo/docker/postgres"
    
    if [ -f "manage.sh" ]; then
        chmod +x manage.sh
        ./manage.sh start
        
        # Дополнительная проверка MinIO
        print_info "Проверка запуска MinIO..."
        sleep 5
        
        if ! docker ps | grep -q pigeongram_minio; then
            print_warning "MinIO не запустился, пробуем запустить отдельно..."
            docker-compose up -d minio
            sleep 5
        fi
        
        if docker ps | grep -q pigeongram_minio; then
            print_success "MinIO запущен"
            curl -s http://localhost:9000/minio/health/live >/dev/null && print_success "MinIO отвечает на запросы"
        else
            print_error "MinIO не удалось запустить. Проверьте логи: docker logs pigeongram_minio"
            docker logs pigeongram_minio --tail 20
        fi
    else
        docker-compose up -d
    fi
    
    print_success "PostgreSQL, Redis и MinIO запущены"
    
    # Синхронизация паролей всех сервисов
    sleep 10
    sync_postgres_password
    sync_redis_password
    sync_minio_password
    
    # Запуск мониторинга
    if [ -d "$APP_DIR/repo/docker/monitoring" ]; then
        print_step "Запуск мониторинга"
        cd "$APP_DIR/repo/docker/monitoring"
        docker-compose up -d
        print_success "Мониторинг запущен"
    fi
    
    # Сборка и запуск приложения
    print_step "Сборка и запуск приложения"
    cd "$APP_DIR/repo"
    
    docker network inspect pigeongram_network >/dev/null 2>&1 || \
        docker network create pigeongram_network
    
    docker build -t pigeongram:latest .
    
    docker stop pigeongram_app 2>/dev/null || true
    docker rm pigeongram_app 2>/dev/null || true
    
    docker run -d \
        --name pigeongram_app \
        --restart unless-stopped \
        -p 8080:8080 \
        --network pigeongram_network \
        -v "$APP_DIR/logs":/app/logs \
        --env-file "$APP_DIR/config/.env.production" \
        pigeongram:latest
    
    print_success "Приложение запущено"
}

# Проверка статуса
check_status() {
    print_step "Проверка статуса"
    
    echo ""
    echo "📊 Контейнеры:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "   Docker не запущен"
    
    echo ""
    echo "📁 Конфигурация:"
    if [ -f "$APP_DIR/config/.env.production" ]; then
        echo "   ✅ Конфигурация найдена"
        echo "   📍 $APP_DIR/config/.env.production"
    else
        echo "   ❌ Конфигурация не найдена"
    fi
    
    echo ""
    SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "unknown")
    echo "🌐 Приложение должно быть доступно по адресу: http://$SERVER_IP:8080"
}

# Показать справку
show_help() {
    print_header "PigeonGram - Полное развертывание"
    echo ""
    echo "  Использование: $0 [КОМАНДА]"
    echo ""
    echo "  Команды:"
    echo "    full        - Полное развертывание (сервер + приложение)"
    echo "    server      - Только установка сервера"
    echo "    app         - Только деплой приложения"
    echo "    status      - Проверить статус"
    echo "    help        - Показать эту справку"
    echo ""
    echo "  Переменные окружения:"
    echo "    DOMAIN          - Домен (по умолчанию: localhost)"
    echo "    GITHUB_TOKEN    - Токен для доступа к GitHub"
    echo "    ENV_FILE        - Путь к файлу .env.production"
    echo ""
    echo "  Примеры:"
    echo "    GITHUB_TOKEN=ghp_xxx ENV_FILE=/home/user/.env.production ./deploy-pigeongram.sh full"
    echo "    GITHUB_TOKEN=ghp_xxx ./deploy-pigeongram.sh full"
    echo ""
}

# Основная логика
main() {
    case "${1:-help}" in
        full)
            check_root
            print_header "Полное развертывание PigeonGram"
            setup_server
            deploy_app
            check_status
            print_success "Развертывание завершено!"
            ;;
        server)
            check_root
            print_header "Установка сервера"
            setup_server
            ;;
        app)
            print_header "Деплой приложения"
            if [ ! -d "$APP_DIR" ]; then
                print_error "Директория $APP_DIR не найдена. Сначала выполните: $0 server"
                exit 1
            fi
            deploy_app
            check_status
            ;;
        status)
            check_status
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