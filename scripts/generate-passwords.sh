#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}   PigeonGram Password Generator${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Функция генерации случайного пароля
generate_password() {
    local length=${1:-32}
    local type=${2:-"full"}
    
    case $type in
        "simple")
            # Буквы и цифры
            tr -dc 'a-zA-Z0-9' < /dev/urandom | fold -w $length | head -n 1
            ;;
        "medium")
            # Буквы, цифры и специальные символы (!@#$%^&*)
            tr -dc 'a-zA-Z0-9!@#$%^&*' < /dev/urandom | fold -w $length | head -n 1
            ;;
        "full")
            # Все печатные символы, кроме проблемных
            tr -dc 'a-zA-Z0-9!@#$%^&*_+-=<>?' < /dev/urandom | fold -w $length | head -n 1
            ;;
        "readable")
            # Только буквы (читаемые)
            tr -dc 'a-zA-Z' < /dev/urandom | fold -w $((length/2)) | head -n 1 | tr '[:upper:]' '[:lower:]'
            echo -n "_"
            tr -dc '0-9' < /dev/urandom | fold -w 4 | head -n 1
            ;;
    esac
}

# Функция генерации JWT секрета (особенно длинный)
generate_jwt_secret() {
    openssl rand -base64 48 | tr -d '\n' | tr -d '=' | tr -d '/' | tr -d '+'
}

# Функция генерации хеша (для будущего использования)
generate_bcrypt_hash() {
    local password=$1
    echo "Хеш для пароля '$password' можно сгенерировать позже с помощью htpasswd или bcrypt CLI"
}

echo -e "${YELLOW}Генерируем пароли для PigeonGram...${NC}\n"

# PostgreSQL пароль
POSTGRES_PASSWORD=$(generate_password 32 "full")
echo -e "${GREEN}✅ PostgreSQL пароль${NC}"
echo "   DB_PASSWORD=${POSTGRES_PASSWORD}"
echo "   Длина: ${#POSTGRES_PASSWORD} символов"
echo ""

# Redis пароль
REDIS_PASSWORD=$(generate_password 32 "full")
echo -e "${GREEN}✅ Redis пароль${NC}"
echo "   REDIS_PASSWORD=${REDIS_PASSWORD}"
echo "   Длина: ${#REDIS_PASSWORD} символов"
echo ""

# MinIO Secret Key
MINIO_SECRET_KEY=$(generate_password 32 "full")
echo -e "${GREEN}✅ MinIO Secret Key${NC}"
echo "   MINIO_SECRET_KEY=${MINIO_SECRET_KEY}"
echo "   Длина: ${#MINIO_SECRET_KEY} символов"
echo ""

# JWT Secret (особенно длинный и сложный)
JWT_SECRET=$(generate_jwt_secret)
echo -e "${GREEN}✅ JWT Secret${NC}"
echo "   JWT_SECRET=${JWT_SECRET}"
echo "   Длина: ${#JWT_SECRET} символов"
echo ""

# Telegram Bot Token (пример, нужно будет заменить на реальный)
TELEGRAM_BOT_TOKEN=$(generate_password 45 "medium")
echo -e "${GREEN}✅ Telegram Bot Token (пример)${NC}"
echo "   TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}"
echo "   ⚠️  Замените на реальный токен от @BotFather"
echo ""

# SMTP пароль (для Gmail App Password)
SMTP_PASSWORD=$(generate_password 16 "simple")
echo -e "${GREEN}✅ SMTP пароль (для Gmail)${NC}"
echo "   SMTP_PASSWORD=${SMTP_PASSWORD}"
echo "   ⚠️  Для Gmail используйте пароль приложения, не основной пароль"
echo ""

# Дополнительные пароли для сервисов
GRAFANA_ADMIN_PASSWORD="admin"  # По умолчанию, рекомендуется изменить
echo -e "${YELLOW}⚠️  Grafana пароль по умолчанию: admin/admin${NC}"
echo "   Рекомендуется изменить после первого входа"
echo ""

# Читаемый пароль для тестового пользователя (опционально)
TEST_USER_PASSWORD=$(generate_password 12 "readable")
echo -e "${BLUE}📝 Тестовый пользователь (опционально)${NC}"
echo "   TEST_USER_PASSWORD=${TEST_USER_PASSWORD}"
echo ""

# Генерация сложного мастер-пароля
MASTER_PASSWORD=$(generate_password 48 "full")
echo -e "${RED}🔐 Мастер-пароль (сохраните в безопасном месте)${NC}"
echo "   MASTER_PASSWORD=${MASTER_PASSWORD}"
echo ""

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}📋 Готовый .env.production блок:${NC}"
echo ""

# Выводим готовый блок для .env.production
cat << EOF
# =====================================================
# PigeonGram - Production конфигурация
# =====================================================

# 🐘 PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=pigeongram
DB_PASSWORD=${POSTGRES_PASSWORD}
DB_NAME=pigeongram
DB_SSLMODE=disable

# 🔴 Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_DB=0
REDIS_TTL=5m
REDIS_PREFIX=pigeongram
REDIS_ENABLED=true

# 📁 MinIO
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
MINIO_USE_SSL=false
MINIO_BUCKET=pigeongram-files
MINIO_REGION=us-east-1
MINIO_UPLOAD_EXPIRY=15m
MINIO_DOWNLOAD_EXPIRY=24h
MINIO_MAX_FILE_SIZE=104857600

# 🚀 Сервер
SERVER_PORT=8080
SERVER_ENVIRONMENT=production
DOMAIN=your-domain.com

# 🔐 Безопасность
SESSION_DURATION=24h
COOKIE_SECURE=true
JWT_SECRET=${JWT_SECRET}

# 📧 Email (для уведомлений)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=${SMTP_PASSWORD}

# 🔔 Telegram алерты (опционально)
TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
TELEGRAM_CHAT_ID=your-chat-id
EOF

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}📝 Инструкция по сохранению:${NC}"
echo "1. Сохраните эти пароли в менеджере паролей (LastPass, 1Password, Bitwarden)"
echo "2. Мастер-пароль храните отдельно, он не используется напрямую"
echo "3. Для Gmail используйте 'App Password', а не основной пароль"
echo "4. Telegram токен получите у @BotFather"
echo "5. Измените пароль Grafana после первого входа"
echo ""

# Сохраняем в файл (опционально)
if [ "$1" == "--save" ]; then
    OUTPUT_FILE="pigeongram-passwords-$(date +%Y%m%d).txt"
    {
        echo "PigeonGram Passwords - $(date)"
        echo "================================="
        echo ""
        echo "PostgreSQL: ${POSTGRES_PASSWORD}"
        echo "Redis: ${REDIS_PASSWORD}"
        echo "MinIO Secret: ${MINIO_SECRET_KEY}"
        echo "JWT Secret: ${JWT_SECRET}"
        echo "SMTP: ${SMTP_PASSWORD}"
        echo "Telegram Bot: ${TELEGRAM_BOT_TOKEN}"
        echo "Master: ${MASTER_PASSWORD}"
        echo ""
        echo "Сохраните в безопасном месте!"
    } > "$OUTPUT_FILE"
    echo -e "${GREEN}✅ Пароли сохранены в файл: $OUTPUT_FILE${NC}"
    echo -e "${RED}⚠️  ВНИМАНИЕ: Удалите файл после сохранения в менеджере паролей!${NC}"
fi