#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Функции для вывода
print_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
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

# Проверка, что скрипт запущен от root
if [[ $EUID -ne 0 ]]; then
   print_error "Этот скрипт должен запускаться от root (sudo)"
   exit 1
fi

print_step "Начало инициализации сервера для PigeonGram"

# 1. Обновление системы
print_step "Обновление пакетов системы"
apt update && apt upgrade -y
print_success "Система обновлена"

# 2. Установка необходимых пакетов
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
    docker.io \
    docker-compose \
    jq \
    ncdu
print_success "Базовые пакеты установлены"

# 3. Настройка Docker
print_step "Настройка Docker"
systemctl enable docker
systemctl start docker
usermod -aG docker $SUDO_USER
print_success "Docker настроен"

# 4. Настройка файрвола
print_step "Настройка файрвола (UFW)"
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'
ufw allow 9090/tcp comment 'Prometheus'
ufw allow 3000/tcp comment 'Grafana'

echo "y" | ufw enable
print_success "Файрвол настроен"

# 5. Настройка автоматических обновлений безопасности
print_step "Настройка автоматических обновлений"
cat > /etc/apt/apt.conf.d/20auto-upgrades << EOF
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF
print_success "Автоматические обновления настроены"

# 6. Настройка fail2ban
print_step "Настройка fail2ban"
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

# 7. Настройка параметров ядра
print_step "Настройка параметров ядра"
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
print_success "Параметры ядра настроены"

# 8. Создание структуры директорий
print_step "Создание структуры директорий"
mkdir -p /opt/pigeongram/{data,logs,backups,ssl,config,scripts}
mkdir -p /opt/pigeongram/data/{postgres,redis,minio,prometheus,grafana}
chmod 755 /opt/pigeongram
print_success "Директории созданы в /opt/pigeongram"

# 9. Установка Docker Compose
print_step "Установка Docker Compose"
COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose
print_success "Docker Compose ${COMPOSE_VERSION} установлен"

# 10. Настройка swap
print_step "Настройка swap"
if [ ! -f /swapfile ]; then
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' | tee -a /etc/fstab
    print_success "Swap файл создан (2GB)"
else
    print_warning "Swap файл уже существует"
fi

# 11. Информация для следующего шага
print_step "Инициализация завершена"
echo ""
echo "📋 Следующие шаги:"
echo "1. Переключитесь на пользователя: su - $SUDO_USER"
echo "2. Скопируйте файлы проекта в /opt/pigeongram"
echo "3. Настройте .env.production файл"
echo "4. Запустите скрипт деплоя: ./scripts/02-deploy-app.sh"
echo ""
echo "📊 Информация о сервере:"
echo "   IP: $(curl -s ifconfig.me)"
echo "   CPU: $(nproc) ядер"
echo "   RAM: $(free -h | grep Mem | awk '{print $2}')"
echo "   Диск: $(df -h / | awk 'NR==2 {print $2}')"

print_success "Готово!"