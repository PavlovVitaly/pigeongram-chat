#!/bin/bash

echo "🛑 Остановка кластера..."

# Останавливаем Go процессы
pkill -f "go run main.go"
pkill -f "main"

# Останавливаем Docker
cd docker/postgres
docker-compose down

echo "✅ Кластер остановлен"