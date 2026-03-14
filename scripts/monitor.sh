#!/bin/bash

echo "📊 Мониторинг кластера PigeonGram"
echo "================================"

# Проверка активных серверов через Redis
echo "🔍 Активные серверы:"
redis-cli -a redis_secret KEYS "server:*"

# Статистика Redis
echo ""
echo "📈 Статистика Redis:"
redis-cli -a redis_secret INFO stats | grep -E "total_connections_received|total_commands_processed"

# Проверка WebSocket соединений
echo ""
echo "🔌 WebSocket соединения:"
for port in 8080 8081 8082; do
    count=$(lsof -i :$port 2>/dev/null | grep ESTABLISHED | wc -l)
    echo "  Сервер $port: $count соединений"
done

echo ""
echo "✅ Мониторинг завершен"