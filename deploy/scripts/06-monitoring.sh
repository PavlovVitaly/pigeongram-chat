#!/bin/bash

PROJECT_ROOT="/opt/pigeongram"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

print_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}ℹ️ $1${NC}"
}

cd $PROJECT_ROOT/repo/docker/monitoring

# Создаем файл для токена MinIO
print_step "Настройка MinIO токена"
touch minio-token
chmod 666 minio-token
print_success "Файл minio-token создан"

# Настраиваем дашборды Grafana
print_step "Настройка Grafana provisioning"
mkdir -p grafana-provisioning/dashboards
mkdir -p grafana-provisioning/datasources

# Создаем datasource
cat > grafana-provisioning/datasources/prometheus.yaml << EOF
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
    jsonData:
      timeInterval: "15s"
EOF

# Копируем дашборд если есть
if [ -f "$PROJECT_ROOT/repo/deploy/config/grafana-dashboard.json" ]; then
    cp "$PROJECT_ROOT/repo/deploy/config/grafana-dashboard.json" grafana-provisioning/dashboards/
    print_success "Дашборд скопирован"
fi

# Запускаем мониторинг через monitor.sh если есть
print_step "Запуск мониторинга"
if [ -f "monitor.sh" ]; then
    chmod +x monitor.sh
    ./monitor.sh start
else
    docker-compose up -d
fi

# Ждем готовности
sleep 5

# Проверяем статус
print_step "Проверка статуса"
if [ -f "monitor.sh" ]; then
    ./monitor.sh status
else
    docker-compose ps
fi

print_success "Мониторинг настроен"
echo ""
echo "📊 Доступные сервисы:"
SERVER_IP=$(curl -s ifconfig.me)
echo "   Grafana:    http://$SERVER_IP:3000 (admin/admin)"
echo "   Prometheus: http://$SERVER_IP:9090"
echo ""
echo "📋 Управление мониторингом:"
echo "   cd $PROJECT_ROOT/repo/docker/monitoring"
echo "   ./monitor.sh help"