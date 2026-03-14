#!/bin/bash

echo "🚀 Запуск кластера PigeonGram..."

# Запускаем инфраструктуру
cd docker/postgres
docker-compose up -d

cd ../..

# Запускаем 3 экземпляра приложения
echo "📦 Запуск сервера 1 (порт 8080)..."
go run main.go -port 8080 -server-id server-1 -use-redis=true &

echo "📦 Запуск сервера 2 (порт 8081)..."
go run main.go -port 8081 -server-id server-2 -use-redis=true &

echo "📦 Запуск сервера 3 (порт 8082)..."
go run main.go -port 8082 -server-id server-3 -use-redis=true &

echo "✅ Кластер запущен!"
echo "🌐 Доступ через Nginx: http://localhost"
echo "📊 Мониторинг Redis: redis-cli -a redis_secret"