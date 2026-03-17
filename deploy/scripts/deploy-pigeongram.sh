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
GITHUB_TOKEN="${GITHUB_TOKEN:-""}"  # 👈 Берем из переменной окружения
APP_USER="app"
APP_DIR="/opt/pigeongram"
DOMAIN=${DOMAIN:-"localhost"}

# Формируем URL для клонирования
if [ -n "$GITHUB_TOKEN" ]; then
    GITHUB_URL="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
else
    GITHUB_URL="https://github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
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

# Функции установки сервера
setup_server() {
    print_step "Установка и настройка сервера"
    
    # Обновление системы
    apt update && apt upgrade -y
    print_success "Система обновлена"
    
    # Установка базовых пакетов
    apt install -y \
        curl wget git vim htop net-tools \
        ufw fail2ban unattended-upgrades \
        jq ncdu build-essential software-properties-common \
        apt-transport-https ca-certificates gnupg lsb-release
    print_success "Базовые пакеты установлены"
    
    # Установка Docker
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    print_success "Docker установлен"
    
    # Установка Docker Compose
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
    curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    print_success "Docker Compose ${COMPOSE_VERSION} установлен"
    
    # Создание пользователя
    if ! id "$APP_USER" &>/dev/null; then
        useradd -m -s /bin/bash "$APP_USER"
        echo "$APP_USER:$(generate_password)" | chpasswd
        print_success "Пользователь $APP_USER создан"
    fi
    
    usermod -aG docker "$APP_USER"
    print_success "Пользователь добавлен в группу docker"
    
    # Создание структуры директорий
    mkdir -p "$APP_DIR"/{repo,data,logs,backups,ssl,config}
    chown -R "$APP_USER":"$APP_USER" "$APP_DIR"
    print_success "Директории созданы в $APP_DIR"
    
    # Настройка файрвола
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
    
    # Настройка fail2ban
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
    
    # Настройка swap
    if [ ! -f /swapfile ]; then
        fallocate -l 2G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
        print_success "Swap файл создан (2GB)"
    fi
    
    # Оптимизация ядра
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
    
    # Генерация паролей
    POSTGRES_PASSWORD=$(generate_password)
    REDIS_PASSWORD=$(generate_password)
    MINIO_PASSWORD=$(generate_password)
    JWT_SECRET=$(generate_jwt_secret)
    
    # Создание .env файла
    cat > "$APP_DIR/config/.env.production" << EOF
# =====================================================
# PigeonGram - Production конфигурация
# =====================================================

# 🐘 PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=pigeongram
DB_PASSWORD=${POSTGRES_PASSWORD}
DB_NAME=pigeongram
DB_SSLMODE=disable

# 🔴 Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_DB=0
REDIS_TTL=5m
REDIS_PREFIX=pigeongram
REDIS_ENABLED=true

# 📁 MinIO
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=${MINIO_PASSWORD}
MINIO_USE_SSL=false
MINIO_BUCKET=pigeongram-files
MINIO_REGION=us-east-1
MINIO_UPLOAD_EXPIRY=15m
MINIO_DOWNLOAD_EXPIRY=24h
MINIO_MAX_FILE_SIZE=104857600

# 🚀 Сервер
SERVER_PORT=8080
SERVER_ENVIRONMENT=production
DOMAIN=${DOMAIN}

# 🔐 Безопасность
SESSION_DURATION=24h
COOKIE_SECURE=false
JWT_SECRET=${JWT_SECRET}
EOF

    chown "$APP_USER":"$APP_USER" "$APP_DIR/config/.env.production"
    chmod 600 "$APP_DIR/config/.env.production"
    print_success "Конфигурация создана"
    
    # Генерация SSL сертификатов
    if [ ! -f "$APP_DIR/ssl/cert.pem" ]; then
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "$APP_DIR/ssl/key.pem" \
            -out "$APP_DIR/ssl/cert.pem" \
            -subj "/C=RU/ST=Moscow/L=Moscow/O=PigeonGram/CN=${DOMAIN}"
        chown -R "$APP_USER":"$APP_USER" "$APP_DIR/ssl"
        print_success "SSL сертификаты сгенерированы"
    fi
    
    # Клонирование репозитория
    print_step "Клонирование репозитория"
    cd "$APP_DIR"
    
    if [ -d "repo" ]; then
        # Проверяем, является ли это git-репозиторием
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
    fi
    
    # Запуск инфраструктуры
    print_step "Запуск PostgreSQL и MinIO"
    cd "$APP_DIR/repo/docker/postgres"
    
    # Проверяем наличие manage.sh
    if [ -f "manage.sh" ]; then
        chmod +x manage.sh
        ./manage.sh start
    else
        docker-compose up -d
    fi
    print_success "PostgreSQL и MinIO запущены"
    
    # Запуск мониторинга (опционально)
    if [ -d "$APP_DIR/repo/docker/monitoring" ]; then
        print_step "Запуск мониторинга"
        cd "$APP_DIR/repo/docker/monitoring"
        docker-compose up -d
        print_success "Мониторинг запущен"
    fi
    
    # Сборка и запуск приложения
    print_step "Сборка и запуск приложения"
    cd "$APP_DIR/repo"
    
    # Создаем Docker сеть
    docker network inspect pigeongram_network >/dev/null 2>&1 || \
        docker network create pigeongram_network
    
    # Собираем образ
    docker build -t pigeongram:latest .
    
    # Запускаем контейнер
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
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "   Docker не запущен"
    
    echo ""
    echo "📁 Директории:"
    ls -la "$APP_DIR" 2>/dev/null || echo "   Директория $APP_DIR не найдена"
    
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
    echo "    GITHUB_TOKEN    - Токен для доступа к GitHub (обязательно для приватных репозиториев)"
    echo ""
    echo "  Примеры:"
    echo "    GITHUB_TOKEN=ghp_xxx ./deploy-pigeongram.sh full"
    echo "    DOMAIN=example.com GITHUB_TOKEN=ghp_xxx ./deploy-pigeongram.sh full"
    echo ""
}

# Основная логика
main() {
    case "${1:-help}" in
        full)
            check_root
            print_header "Полное развертывание PigeonGram"
            
            if [ -z "$GITHUB_TOKEN" ]; then
                print_warning "GITHUB_TOKEN не указан. Если репозиторий приватный, клонирование не удастся."
                read -p "Продолжить без токена? (y/N) " -n 1 -r
                echo
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    exit 1
                fi
            fi
            
            setup_server
            deploy_app
            check_status
            print_success "Развертывание завершено!"
            echo ""
            echo "📝 Сохраните эти пароли в безопасном месте:"
            echo "   PostgreSQL пароль: $POSTGRES_PASSWORD"
            echo "   Redis пароль: $REDIS_PASSWORD"
            echo "   MinIO пароль: $MINIO_PASSWORD"
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