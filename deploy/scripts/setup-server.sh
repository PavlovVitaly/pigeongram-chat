#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Функции для вывода
print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}   $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
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

print_header "Начало установки PigeonGram на сервер"

# 1. Обновление системы
print_header "Обновление пакетов системы"
apt update && apt upgrade -y
print_success "Система обновлена"

# 2. Установка базовых пакетов
print_header "Установка базовых пакетов"
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
    lsb-release
print_success "Базовые пакеты установлены"

# 3. Установка Docker
print_header "Установка Docker"
# Удаление старых версий если есть
apt remove -y docker docker-engine docker.io containerd runc 2>/dev/null

# Добавление официального GPG ключа Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Добавление репозитория
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# Установка Docker
apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
print_success "Docker установлен"

# 4. Установка Docker Compose
print_header "Установка Docker Compose"
COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose
print_success "Docker Compose ${COMPOSE_VERSION} установлен"

# 5. Настройка Docker
print_header "Настройка Docker"
systemctl enable docker
systemctl start docker
print_success "Docker настроен и запущен"

# 6. Создание пользователя для приложения
print_header "Создание пользователя для приложения"
if id "app" &>/dev/null; then
    print_warning "Пользователь app уже существует"
else
    useradd -m -s /bin/bash app
    echo "app:$(openssl rand -base64 12)" | chpasswd
    print_success "Пользователь app создан"
fi

# Добавление пользователя в группу docker
usermod -aG docker app
print_success "Пользователь app добавлен в группу docker"

# 7. Создание структуры директорий
print_header "Создание структуры директорий"
mkdir -p /opt/pigeongram/{repo,data,logs,backups,ssl,config}
mkdir -p /opt/pigeongram/data/{postgres,redis,minio,prometheus,grafana}
chown -R app:app /opt/pigeongram
chmod 755 /opt/pigeongram
print_success "Директории созданы в /opt/pigeongram"

# 8. Настройка файрвола
print_header "Настройка файрвола (UFW)"
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

# Включение файрвола (с подтверждением)
echo "y" | ufw enable
print_success "Файрвол настроен"

# 9. Настройка автоматических обновлений безопасности
print_header "Настройка автоматических обновлений"
cat > /etc/apt/apt.conf.d/20auto-upgrades << EOF
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF
print_success "Автоматические обновления настроены"

# 10. Настройка fail2ban для защиты SSH
print_header "Настройка fail2ban"
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

# 11. Настройка swap (если мало RAM)
print_header "Настройка swap"
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

# 12. Настройка параметров ядра
print_header "Настройка параметров ядра"
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

# 13. Установка и настройка Node Exporter для мониторинга (опционально)
print_header "Установка Node Exporter для мониторинга"
NODE_EXPORTER_VERSION="1.6.1"
wget https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz
tar xvfz node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz
mv node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter /usr/local/bin/
rm -rf node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64*

# Создание systemd сервиса для node_exporter
cat > /etc/systemd/system/node_exporter.service << EOF
[Unit]
Description=Node Exporter
After=network.target

[Service]
User=app
Type=simple
ExecStart=/usr/local/bin/node_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable node_exporter
systemctl start node_exporter
print_success "Node Exporter установлен и запущен"

# 14. Проверка установленных версий
print_header "Проверка установленных версий"
echo "Docker: $(docker --version)"
echo "Docker Compose: $(docker-compose --version)"
echo "Git: $(git --version)"
echo "Node Exporter: $(node_exporter --version 2>&1 | head -n1)"

# 15. Информация для следующего шага
print_header "Установка завершена!"
echo ""
echo "📋 Следующие шаги:"
echo "1. Переключитесь на пользователя: su - app"
echo "2. Склонируйте репозиторий: git clone https://github.com/yourusername/pigeongram.git"
echo "3. Настройте .env файл"
echo "4. Запустите приложение: docker-compose up -d"
echo ""
echo "📊 Информация о сервере:"
echo "   IP: $(curl -s ifconfig.me)"
echo "   CPU: $(nproc) ядер"
echo "   RAM: $(free -h | grep Mem | awk '{print $2}')"
echo "   Диск: $(df -h / | awk 'NR==2 {print $2}')"
echo ""
echo "🔐 Доступные порты:"
echo "   22   - SSH"
echo "   80   - HTTP"
echo "   443  - HTTPS"
echo "   8080 - PigeonGram App"
echo "   9090 - Prometheus"
echo "   3000 - Grafana"
echo "   9000 - MinIO API"
echo "   9001 - MinIO Console"
echo ""
print_success "Готово! Сервер готов к развертыванию PigeonGram"