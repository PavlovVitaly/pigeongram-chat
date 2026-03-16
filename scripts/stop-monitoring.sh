#!/bin/bash

echo "🛑 Остановка системы мониторинга"

cd docker/monitoring
docker-compose down

echo "✅ Мониторинг остановлен"