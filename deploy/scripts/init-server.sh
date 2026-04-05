#!/bin/bash
# =====================================================
# PigeonGram - Начальная инициализация сервера
# =====================================================

set -e

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_step() {
    echo -e "\n${GREEN}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️ $1${NC}"
}

# Проверка root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "Этот скрипт должен запускаться от root (sudo)"
        exit 1
    fi
}

# =====================================================
# 1. Обновление системы
# =====================================================
update_system() {
    print_step "Обновление системы"
    
    apt update
    apt upgrade -y
    apt autoremove -y
    
    print_success "Система обновлена"
}

# =====================================================
# 2. Установка базовых пакетов
# =====================================================
install_base_packages() {
    print_step "Установка базовых пакетов"
    
    apt install -y \
        curl \
        wget \
        git \
        vim \
        htop \
        net-tools \
        ufw \
        fail2ban \
        unattended-upgrades \
        jq \
        ncdu \
        build-essential \
        software-properties-common \
        apt-transport-https \
        ca-certificates \
        gnupg \
        lsb-release \
        tree \
        zip \
        unzip \
        mc \
        rsync \
        tmux \
        iotop \
        iftop \
	openssl
    
    print_success "Базовые пакеты установлены"
}

# =====================================================
# 3. Установка Docker
# =====================================================
install_docker() {
    print_step "Установка Docker"
    
    # Проверка, установлен ли Docker
    if command -v docker &> /dev/null; then
        print_info "Docker уже установлен"
        docker --version
        return 0
    fi
    
    # Установка Docker
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    
    # Добавление пользователя в группу docker
    if id "app" &>/dev/null; then
        usermod -aG docker app
        print_info "Пользователь app добавлен в группу docker"
    fi
    
    # Включение автозапуска
    systemctl enable docker
    systemctl start docker
    
    print_success "Docker установлен"
    docker --version
}

# =====================================================
# 4. Установка Docker Compose
# =====================================================
install_docker_compose() {
    print_step "Установка Docker Compose"
    
    # Проверка, установлен ли Docker Compose
    if command -v docker-compose &> /dev/null; then
        print_info "Docker Compose уже установлен"
        docker-compose --version
        return 0
    fi
    
    # Установка последней версии
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
    curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose
    
    print_success "Docker Compose установлен"
    docker-compose --version
}

# =====================================================
# 5. Установка Go
# =====================================================
install_go() {
    print_step "Установка Go"
    
    # Проверка, установлен ли Go
    if command -v go &> /dev/null; then
        print_info "Go уже установлен"
        go version
        return 0
    fi
    
    # Установка Go 1.26
    GO_VERSION="1.26.1"
    ARCH=$(uname -m)
    
    if [ "$ARCH" = "x86_64" ]; then
        GO_ARCH="amd64"
    elif [ "$ARCH" = "aarch64" ]; then
        GO_ARCH="arm64"
    else
        GO_ARCH="amd64"
    fi
    
    wget "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    rm "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    
    # Добавление в PATH
    if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    
    # Создание директории для Go workspace
    mkdir -p /root/go
    echo 'export GOPATH=$HOME/go' >> /root/.bashrc
    echo 'export PATH=$PATH:$GOPATH/bin' >> /root/.bashrc
    
    export PATH=$PATH:/usr/local/go/bin
    
    print_success "Go установлен"
    /usr/local/go/bin/go version
}

# =====================================================
# 6. Создание пользователя для приложения
# =====================================================
create_app_user() {
    print_step "Создание пользователя app"
    
    if id "app" &>/dev/null; then
        print_info "Пользователь app уже существует"
        return 0
    fi
    
    useradd -m -s /bin/bash app
    echo "app:$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)" | chpasswd
    
    # Добавление в группу docker
    usermod -aG docker app
    
    print_success "Пользователь app создан"
}

# =====================================================
# 7. Настройка фаервола (UFW)
# =====================================================
setup_firewall() {
    print_step "Настройка фаервола"
    
    # Сброс правил
    ufw --force reset
    
    # Базовые правила
    ufw default deny incoming
    ufw default allow outgoing
    
    # Разрешаем SSH
    ufw allow 22/tcp comment 'SSH'
    
    # Разрешаем HTTP/HTTPS
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'
    
    # Разрешаем приложение
    ufw allow 8080/tcp comment 'PigeonGram App'
    
    # Разрешаем MinIO
    ufw allow 9000/tcp comment 'MinIO API'
    ufw allow 9001/tcp comment 'MinIO Console'
    
    # Разрешаем мониторинг
    ufw allow 9090/tcp comment 'Prometheus'
    ufw allow 3000/tcp comment 'Grafana'
    
    # Включаем фаервол (автоматически отвечаем yes)
    echo "y" | ufw enable
    
    print_success "Фаервол настроен"
    ufw status verbose
}

# =====================================================
# 8. Настройка fail2ban
# =====================================================
setup_fail2ban() {
    print_step "Настройка fail2ban"
    
    # Создание конфигурации
    cat > /etc/fail2ban/jail.local << 'EOF'
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5
destemail = root@localhost
action = %(action_mwl)s

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 3600

[nginx-http-auth]
enabled = true
port = http,https
filter = nginx-http-auth
logpath = /var/log/nginx/error.log
maxretry = 5
bantime = 3600
EOF

    systemctl restart fail2ban
    systemctl enable fail2ban
    
    print_success "fail2ban настроен"
}

# =====================================================
# 9. Настройка автоматических обновлений
# =====================================================
setup_auto_updates() {
    print_step "Настройка автоматических обновлений"
    
    cat > /etc/apt/apt.conf.d/20auto-upgrades << 'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF
    
    cat > /etc/apt/apt.conf.d/50unattended-upgrades << 'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}";
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
EOF

    systemctl restart unattended-upgrades
    
    print_success "Автоматические обновления настроены"
}

# =====================================================
# 10. Настройка swap
# =====================================================
setup_swap() {
    print_step "Настройка swap"
    
    # Проверка, существует ли swap
    if swapon --show | grep -q "/swapfile"; then
        print_info "Swap уже настроен"
        return 0
    fi
    
    # Создание swap файла (2GB)
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    
    # Добавление в fstab
    echo '/swapfile none swap sw 0 0' | tee -a /etc/fstab
    
    # Настройка swappiness
    sysctl vm.swappiness=10
    echo "vm.swappiness=10" >> /etc/sysctl.conf
    
    print_success "Swap создан (2GB)"
}

# =====================================================
# 11. Оптимизация параметров ядра
# =====================================================
setup_kernel_params() {
    print_step "Оптимизация параметров ядра"
    
    cat >> /etc/sysctl.conf << 'EOF'
# PigeonGram optimizations
net.core.somaxconn = 1024
net.ipv4.tcp_max_syn_backlog = 4096
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
vm.overcommit_memory = 1
EOF

    sysctl -p
    
    print_success "Параметры ядра оптимизированы"
}

# =====================================================
# 12. Создание директорий для проекта
# =====================================================
create_directories() {
    print_step "Создание директорий для проекта"
    
    mkdir -p /opt/pigeongram
    chown -R app:app /opt/pigeongram
    chmod 755 /opt/pigeongram
    
    print_success "Директории созданы"
}

# =====================================================
# 13. Проверка установки
# =====================================================
verify_installation() {
    print_step "Проверка установки"
    
    echo ""
    echo "📦 Установленные версии:"
    echo "   Docker: $(docker --version 2>/dev/null || echo 'не установлен')"
    echo "   Docker Compose: $(docker-compose --version 2>/dev/null || echo 'не установлен')"
    echo "   Go: $(/usr/local/go/bin/go version 2>/dev/null || echo 'не установлен')"
    echo "   Git: $(git --version 2>/dev/null || echo 'не установлен')"
    echo ""
    
    echo "🔌 Службы:"
    echo "   Docker: $(systemctl is-active docker)"
    echo "   fail2ban: $(systemctl is-active fail2ban)"
    echo "   unattended-upgrades: $(systemctl is-active unattended-upgrades)"
    echo ""
    
    echo "🌐 Открытые порты (UFW):"
    ufw status | grep -E "ALLOW|22|80|443|8080|9000|9001|9090|3000" || echo "   Правила не найдены"
}

# =====================================================
# Главная функция
# =====================================================
main() {
    print_header "PigeonGram - Начальная инициализация сервера"
    
    check_root
    
    update_system
    install_base_packages
    install_docker
    install_docker_compose
    install_go
    create_app_user
    setup_firewall
    setup_fail2ban
    setup_auto_updates
    setup_swap
    setup_kernel_params
    create_directories
    verify_installation
    
    print_header "Инициализация завершена!"

    echo "🔧 Команды для проверки:"
    echo "   docker ps                     # статус контейнеров"
    echo "   docker-compose logs -f        # логи"
    echo ""
}

# Запуск
main "$@"
