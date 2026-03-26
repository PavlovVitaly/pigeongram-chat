#!/bin/bash
# Скрипт для создания bucket и настройки MinIO

set -e

echo "⏳ Ожидание запуска MinIO..."
sleep 5

# Создание bucket
echo "📦 Создание bucket 'pigeongram-files'..."
docker exec pigeongram_minio mc alias set local http://localhost:9000 minioadmin minioadmin
docker exec pigeongram_minio mc mb local/pigeongram-files --ignore-existing

echo "✅ MinIO инициализирован"