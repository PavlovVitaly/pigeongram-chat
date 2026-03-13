#!/bin/bash

echo "🔍 Тестирование Redis интеграции..."

# 1. Проверяем, что Redis запущен
redis-cli -a redis_secret ping
if [ $? -eq 0 ]; then
    echo "✅ Redis работает"
else
    echo "❌ Redis не отвечает"
    exit 1
fi

# 2. Очищаем Redis
redis-cli -a redis_secret FLUSHALL
echo "🧹 Redis очищен"

# 3. Запускаем приложение с Redis
echo "🚀 Запуск PigeonGram с Redis..."
go run main.go -use-redis=true &
APP_PID=$!

# 4. Ждем 5 секунд
sleep 5

# 5. Проверяем ключи в Redis
KEYS=$(redis-cli -a redis_secret KEYS "*")
echo "📊 Ключи в Redis:"
echo "$KEYS"

# 6. Останавливаем приложение
kill $APP_PID

echo "✅ Тест завершен"