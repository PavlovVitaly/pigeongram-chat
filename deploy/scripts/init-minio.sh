#!/bin/bash
# Скрипт для создания bucket и настройки MinIO

set -e

echo "⏳ Ожидание запуска MinIO..."
sleep 5

# Проверка, что MinIO готов
for i in {1..30}; do
    if docker exec pigeongram_minio curl -s http://localhost:9000/minio/health/live > /dev/null 2>&1; then
        echo "✅ MinIO готов"
        break
    fi
    echo "⏳ Ожидание MinIO... ($i/30)"
    sleep 2
done

# Создание bucket
echo "📦 Создание bucket 'pigeongram-files'..."
docker exec pigeongram_minio mc alias set local http://localhost:9000 minioadmin minioadmin 2>/dev/null || true
docker exec pigeongram_minio mc mb local/pigeongram-files --ignore-existing 2>/dev/null || true

echo "✅ MinIO инициализирован"