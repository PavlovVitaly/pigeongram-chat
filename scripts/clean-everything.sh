#!/bin/bash

echo "🧹 Полная очистка всех данных PigeonGram"

# 1. Очистка PostgreSQL
echo "📦 Очистка PostgreSQL..."
cd docker/postgres
docker-compose exec -T postgres psql -U pigeongram -d pigeongram -c "TRUNCATE messages RESTART IDENTITY CASCADE;"
docker-compose exec -T postgres psql -U pigeongram -d pigeongram -c "DELETE FROM users WHERE username IN ('test', 'admin');"

# 2. Очистка Redis
echo "🔴 Очистка Redis..."
docker exec pigeongram_redis redis-cli -a redis_secret FLUSHALL

# 3. Очистка MinIO (если есть файлы)
echo "📁 Очистка MinIO..."
docker exec pigeongram_minio rm -rf /data/* 2>/dev/null || true

# 4. Проверка
echo ""
echo "✅ Проверка:"
docker exec pigeongram_postgres psql -U pigeongram -d pigeongram -c "SELECT COUNT(*) FROM messages;" | grep "0" && echo "   PostgreSQL: чисто"
docker exec pigeongram_redis redis-cli -a redis_secret KEYS "*" | wc -l | grep "0" && echo "   Redis: чисто"

echo "✅ Готово!"