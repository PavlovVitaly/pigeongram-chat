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

print_header() { echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}\n${BLUE}   $1${NC}\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"; }
print_step() { echo -e "\n${CYAN}▶ $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠️ $1${NC}"; }
print_info() { echo -e "${PURPLE}ℹ️ $1${NC}"; }

# Генерация паролей
generate_password() { openssl rand -base64 32 | tr -d '/+=' | cut -c1-32; }
generate_jwt_secret() { openssl rand -base64 48 | tr -d '/+=' | cut -c1-64; }

# Функция поиска файла конфигурации
find_env_file() {
    if [ -n "$ENV_FILE" ] && [ -f "$ENV_FILE" ]; then
        echo -e "${GREEN}✅ Использую конфигурацию: $ENV_FILE${NC}"
        ENV_PATH="$ENV_FILE"
        return 0
    fi
    
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    for file in "$SCRIPT_DIR/.env.production" "$SCRIPT_DIR/.env" "$HOME/.pigeongram.env"; do
        if [ -f "$file" ]; then
            echo -e "${GREEN}✅ Найден конфиг: $file${NC}"
            ENV_PATH="$file"
            return 0
        fi
    done
    
    echo -e "${RED}❌ Файл конфигурации не найден!${NC}"
    return 1
}

# Загрузка переменных из .env
load_env() {
    if [ -f "$1" ]; then
        export $(grep -v '^#' "$1" | xargs)
        return 0
    fi
    return 1
}

# Проверка сервиса
check_service() {
    if docker ps | grep -q "pigeongram_$1"; then
        return 0
    fi
    return 1
}

# Ожидание сервиса
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

# Проверка root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "Запустите от root (sudo)"
        exit 1
    fi
}

# Установка сервера
setup_server() {
    print_step "Установка и настройка сервера"
    
    apt update && apt upgrade -y
    print_success "Система обновлена"
    
    apt install -y curl wget git vim htop net-tools ufw fail2ban unattended-upgrades \
        jq ncdu build-essential software-properties-common apt-transport-https \
        ca-certificates gnupg lsb-release
    print_success "Базовые пакеты установлены"
    
    # Docker
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    print_success "Docker установлен"
    
    # Docker Compose
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
    curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    print_success "Docker Compose ${COMPOSE_VERSION} установлен"
    
    # Пользователь
    if ! id "$APP_USER" &>/dev/null; then
        useradd -m -s /bin/bash "$APP_USER"
        echo "$APP_USER:$(generate_password)" | chpasswd
        print_success "Пользователь $APP_USER создан"
    fi
    usermod -aG docker "$APP_USER"
    
    # Директории
    mkdir -p "$APP_DIR"/{repo,data,logs,backups,ssl,config}
    chown -R "$APP_USER":"$APP_USER" "$APP_DIR"
    
    # Файрвол
    ufw default deny incoming
    ufw default allow outgoing
    for port in 22 80 443 8080 9090 3000 9000 9001; do
        ufw allow $port/tcp
    done
    echo "y" | ufw enable
    print_success "Файрвол настроен"
    
    # fail2ban
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
    
    # Swap
    if [ ! -f /swapfile ]; then
        fallocate -l 2G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
    fi
    
    # sysctl
    cat >> /etc/sysctl.conf << EOF
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

# 👇 ФУНКЦИЯ ПРОВЕРКИ И НАСТРОЙКИ NGINX
setup_nginx_hosts() {
    print_step "Настройка Nginx hosts"
    
    # Проверяем, существует ли контейнер Nginx
    if docker ps -a | grep -q "pigeongram_nginx"; then
        print_info "Удаляем старый контейнер Nginx..."
        docker stop pigeongram_nginx 2>/dev/null || true
        docker rm pigeongram_nginx 2>/dev/null || true
    fi
    
    # Получаем IP приложения
    local app_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_app 2>/dev/null)
    
    if [ -z "$app_ip" ]; then
        print_error "Не удалось получить IP приложения"
        return 1
    fi
    
    print_info "IP приложения: $app_ip"
    
    # Создаём новую конфигурацию Nginx с правильным IP
    cd "$APP_DIR/repo/docker/nginx"
    
    cat > nginx.conf << EOF
events {
    worker_connections 1024;
}

http {
    upstream app_servers {
        least_conn;
        server $app_ip:8080;
        keepalive 32;
    }

    server {
        listen 80;
        server_name _;

        client_max_body_size 100M;

        location /static/ {
            proxy_pass http://$app_ip:8080/static/;
            proxy_set_header Host \$host;
        }

        location /ws {
            proxy_pass http://$app_ip:8080/ws;
            proxy_http_version 1.1;
            proxy_set_header Upgrade \$http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host \$host;
            proxy_read_timeout 300s;
        }

        location /minio/ {
            proxy_pass http://minio:9000/;
            proxy_set_header Host \$host;
            client_max_body_size 100M;
            proxy_request_buffering off;
        }

        location / {
            proxy_pass http://$app_ip:8080;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
        }
    }
}
EOF

    # Собираем и запускаем Nginx
    cd "$APP_DIR/repo/docker/postgres"
    docker-compose up -d nginx
    
    # Проверяем
    sleep 3
    if docker ps | grep -q "pigeongram_nginx"; then
        print_success "Nginx запущен"
        
        # Проверяем, что отвечает
        if curl -s -o /dev/null -w "%{http_code}" http://localhost | grep -q "200"; then
            print_success "HTTP сервер отвечает на порту 80"
        else
            print_warning "HTTP сервер не отвечает"
        fi
    else
        print_error "Nginx не запустился"
        docker logs pigeongram_nginx --tail 20
    fi
}

# Деплой приложения
deploy_app() {
    print_step "Деплой приложения"
    
    # 👇 ОЧИСТКА СТАРОЙ СЕТИ
    print_info "Очистка старой сети..."
    docker network rm pigeongram_network 2>/dev/null || true

    SERVER_IP=$(curl -s ifconfig.me)
    export SERVER_IP
    export DOMAIN=${DOMAIN:-$SERVER_IP}

    # Поиск конфигурации
    if ! find_env_file; then
        exit 1
    fi
    
    # Загрузка переменных
    load_env "$ENV_PATH"
    
    # Копирование конфига
    cp "$ENV_PATH" "$APP_DIR/config/.env.production"
    chmod 600 "$APP_DIR/config/.env.production"
    chown "$APP_USER":"$APP_USER" "$APP_DIR/config/.env.production"
    print_success "Конфигурация скопирована"
    
    # SSL сертификаты
    if [ ! -f "$APP_DIR/ssl/cert.pem" ]; then
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "$APP_DIR/ssl/key.pem" \
            -out "$APP_DIR/ssl/cert.pem" \
            -subj "/C=RU/ST=Moscow/L=Moscow/O=PigeonGram/CN=${DOMAIN}"
        chown -R "$APP_USER":"$APP_USER" "$APP_DIR/ssl"
        print_success "SSL сертификаты сгенерированы"
    fi
    
    # 👇 КЛОНИРОВАНИЕ РЕПОЗИТОРИЯ
    print_step "Клонирование репозитория"
    cd "$APP_DIR"
    
    # Формируем URL
    if [ -n "$GITHUB_TOKEN" ]; then
        GITHUB_URL="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
    else
        GITHUB_URL="https://github.com/${GITHUB_USER}/${GITHUB_REPO}.git"
    fi
    
    print_info "URL: $GITHUB_URL"
    
    # Проверка переменных
    if [ -z "$GITHUB_USER" ] || [ -z "$GITHUB_REPO" ]; then
        print_error "GITHUB_USER или GITHUB_REPO не заданы"
        exit 1
    fi
    
    # Клонирование
    if [ -d "repo" ]; then
        if [ -d "repo/.git" ]; then
            print_info "Репозиторий уже существует, обновляем..."
            cd repo && git pull
        else
            print_warning "Директория repo существует, удаляем..."
            rm -rf repo
            sudo -u "$APP_USER" git clone "$GITHUB_URL" repo
        fi
    else
        print_info "Клонирование репозитория..."
        sudo -u "$APP_USER" git clone "$GITHUB_URL" repo
        
        if [ $? -ne 0 ]; then
            print_error "Ошибка клонирования!"
            print_info "Проверьте:"
            echo "   GITHUB_USER=${GITHUB_USER}"
            echo "   GITHUB_REPO=${GITHUB_REPO}"
            echo "   GITHUB_TOKEN=${GITHUB_TOKEN}"
            exit 1
        fi
    fi
    
    # Проверка результата
    if [ ! -d "$APP_DIR/repo" ]; then
        print_error "Репозиторий не склонирован!"
        exit 1
    fi
    print_success "Репозиторий готов"
    
    # 👇 СОЗДАНИЕ СЕТИ
    print_step "Настройка сети"
    if docker network inspect pigeongram_network >/dev/null 2>&1; then
        print_success "Сеть pigeongram_network уже существует"
    else
        print_info "Создание сети pigeongram_network..."
        docker network create pigeongram_network
        print_success "Сеть создана"
    fi
    
    # 👇 ЗАПУСК ИНФРАСТРУКТУРЫ
    print_step "Запуск PostgreSQL, Redis, MinIO и Nginx"
    
    if [ ! -d "$APP_DIR/repo/docker/postgres" ]; then
        print_error "Директория docker/postgres не найдена!"
        ls -la "$APP_DIR/repo/docker/"
        exit 1
    fi
    
    cd "$APP_DIR/repo/docker/postgres"

    export SERVER_IP
    print_info "CORS будет разрешать: http://$SERVER_IP:8080"
    
    # Получаем пароли
    REDIS_PASS=$(grep REDIS_PASSWORD "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    MINIO_PASS=$(grep MINIO_SECRET_KEY "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    DB_PASS=$(grep DB_PASSWORD "$APP_DIR/config/.env.production" | cut -d'=' -f2 | tr -d ' ' | tr -d '\n' | tr -d '\r')
    
    print_info "Redis пароль: $REDIS_PASS"
    print_info "MinIO пароль: $MINIO_PASS"
    
    # Полная очистка
    print_info "Очистка старых данных..."
    docker-compose down -v 2>/dev/null
    
    # Запуск контейнеров
    print_info "Запуск контейнеров..."
    docker-compose up -d
    
    # 👇 ПРОВЕРКА NGINX
    print_info "Проверка Nginx..."
    sleep 5
    
    if docker ps | grep -q "pigeongram_nginx"; then
        print_success "Nginx запущен"
        
        # Проверяем, что Nginx видит приложение (пока приложение ещё не запущено, это нормально)
        if docker exec pigeongram_nginx ping -c 1 pigeongram_app >/dev/null 2>&1; then
            print_success "Nginx видит приложение"
        else
            print_warning "Nginx не видит приложение (это нормально, приложение ещё не запущено)"
        fi
    else
        print_error "Nginx не запущен"
    fi
    
    # 👇 ПОДКЛЮЧЕНИЕ КОНТЕЙНЕРОВ К СЕТИ
    print_info "Подключение контейнеров к сети..."
    for container in pigeongram_postgres pigeongram_redis pigeongram_minio; do
        if docker ps | grep -q "$container"; then
            docker network connect pigeongram_network "$container" 2>/dev/null && \
                print_success "$container подключен к сети" || \
                print_warning "$container уже в сети"
        fi
    done
    
    # Ожидание
    print_info "Ожидание запуска (20 секунд)..."
    sleep 20
    
    # 👇 НАСТРОЙКА ПАРОЛЕЙ
    print_step "Настройка паролей"
    
    # Redis
    if [ -n "$REDIS_PASS" ]; then
        print_info "Настройка Redis..."
        docker exec pigeongram_redis redis-cli CONFIG SET requirepass "$REDIS_PASS" 2>/dev/null || \
        docker exec pigeongram_redis redis-cli -a "$REDIS_PASS" CONFIG SET requirepass "$REDIS_PASS" 2>/dev/null
        
        if docker exec pigeongram_redis redis-cli -a "$REDIS_PASS" PING 2>/dev/null | grep -q "PONG"; then
            print_success "Redis настроен"
        fi
    fi
    
    # MinIO
    if [ -n "$MINIO_PASS" ]; then
        print_info "Настройка MinIO..."
        docker exec pigeongram_minio mc alias set myminio http://localhost:9000 minioadmin "$MINIO_PASS" 2>/dev/null
        print_success "MinIO настроен"
    fi
    
    # PostgreSQL
    if [ -n "$DB_PASS" ]; then
        print_info "Настройка PostgreSQL..."
        docker exec -i pigeongram_postgres psql -U postgres -c "ALTER USER pigeongram WITH PASSWORD '$DB_PASS';" 2>/dev/null
        print_success "PostgreSQL настроен"
    fi
    
    # 👇 МОНИТОРИНГ
    if [ -d "$APP_DIR/repo/docker/monitoring" ]; then
        print_step "Запуск мониторинга"
        cd "$APP_DIR/repo/docker/monitoring"
        docker-compose up -d
        
        # Подключаем мониторинг к сети
        for container in pigeongram_prometheus pigeongram_grafana; do
            if docker ps | grep -q "$container"; then
                docker network connect pigeongram_network "$container" 2>/dev/null || true
            fi
        done
        print_success "Мониторинг запущен"
    fi
    
    # 👇 ПРИЛОЖЕНИЕ
    print_step "Сборка и запуск приложения"
    cd "$APP_DIR/repo"
    
    docker build -t pigeongram:latest .
    
    # Получаем IP приложения для Nginx
    APP_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pigeongram_app 2>/dev/null || echo "")
    
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
    
    # 👇 ЖДЁМ, ПОКА ПРИЛОЖЕНИЕ ЗАПУСТИТСЯ
    print_info "Ожидание запуска приложения..."
    sleep 10
    for i in {1..10}; do
        if docker ps | grep -q "pigeongram_app.*Up"; then
            print_success "Приложение запущено"
            break
        fi
        sleep 2
    done

    setup_nginx_hosts
    
    # 👇 ОБНОВЛЯЕМ HOSTS В NGINX (добавляем приложение)
    if [ -n "$APP_IP" ] && docker ps | grep -q "pigeongram_nginx"; then
        print_info "Обновляем /etc/hosts в Nginx..."
        # Добавляем запись (удаляем старую если есть)
        docker exec --privileged pigeongram_nginx sh -c "sed -i '/pigeongram_app/d' /etc/hosts"
        docker exec --privileged pigeongram_nginx sh -c "echo '$APP_IP pigeongram_app' >> /etc/hosts"
        docker restart pigeongram_nginx
        sleep 2
        print_success "Nginx обновлён"
    fi
    
    # Финальная проверка
    print_step "Проверка подключений"
    sleep 5
    
    # Проверяем, что приложение видит PostgreSQL
    if docker exec pigeongram_app ping -c 1 postgres >/dev/null 2>&1; then
        print_success "Приложение видит PostgreSQL"
    else
        print_warning "Приложение не видит PostgreSQL"
    fi
    
    # Проверяем, что Nginx видит приложение
    if docker ps | grep -q "pigeongram_nginx"; then
        if docker exec pigeongram_nginx ping -c 1 pigeongram_app >/dev/null 2>&1; then
            print_success "Nginx видит приложение"
        else
            print_warning "Nginx не видит приложение"
        fi
    fi
    
    # Проверяем HTTP доступность
    if curl -s -o /dev/null -w "%{http_code}" http://localhost | grep -q "200"; then
        print_success "HTTP сервер отвечает (порт 80)"
    else
        print_warning "HTTP сервер не отвечает на порту 80"
        print_info "Попробуйте: curl -I http://$SERVER_IP"
    fi
}

# Проверка статуса
check_status() {
    print_step "Проверка статуса"
    echo ""
    echo "📊 Контейнеры:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "pigeongram|postgres|redis|minio" || echo "   Сервисы не запущены"
    
    echo ""
    echo "🔍 Сеть:"
    docker network inspect pigeongram_network | grep -A 15 "Containers" || echo "   Сеть не найдена"
    
    SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "unknown")
    echo -e "\n🌐 Приложение: http://$SERVER_IP:8080"
}

# Справка
show_help() {
    print_header "PigeonGram - Полное развертывание"
    echo "  Использование: $0 {full|server|app|status}"
    echo ""
    echo "  Переменные окружения:"
    echo "    DOMAIN          - Домен (по умолчанию: localhost)"
    echo "    GITHUB_TOKEN    - Токен GitHub"
    echo "    ENV_FILE        - Путь к .env.production"
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
                print_error "Сначала выполните: $0 server"
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

main "$@"