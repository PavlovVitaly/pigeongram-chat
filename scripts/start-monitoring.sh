#!/bin/bash

echo "📊 Запуск системы мониторинга PigeonGram"

# Переходим в директорию мониторинга
cd docker/monitoring

# Создаем необходимые директории
mkdir -p grafana-provisioning/datasources
mkdir -p grafana-provisioning/dashboards

# Запускаем контейнеры
docker-compose up -d

# Проверяем статус
echo "✅ Мониторинг запущен:"
echo "   Prometheus: http://localhost:9090"
echo "   Grafana: http://localhost:3000 (admin/admin)"
echo "   Node Exporter: http://localhost:9100"
echo "   cAdvisor: http://localhost:8081"

# Открываем Grafana в браузере
sleep 3
open http://localhost:3000 2>/dev/null || true