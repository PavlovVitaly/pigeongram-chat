#!/bin/bash
# =====================================================
# PigeonGram - Настройка SSL сертификатов (Let's Encrypt)
# =====================================================

set -e

DOMAIN="test.pigeongram.com.ru"
TEST_DOMAIN="test.pigeongram.com.ru"
EMAIL="admin@test.pigeongram.com.ru"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_step() { echo -e "\n${BLUE}▶ $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }

print_step "Настройка SSL сертификатов для $DOMAIN"

# Создаем директории
mkdir -p certbot/www
mkdir -p certificates/nginx/ssl

# Проверяем DNS
print_step "Проверка DNS записей..."
for domain in $DOMAIN $TEST_DOMAIN; do
    IP=$(dig +short $domain)
    if [ -z "$IP" ]; then
        print_error "DNS запись для $domain не найдена"
        exit 1
    fi
    print_success "$domain -> $IP"
done

# Запускаем nginx (только HTTP)
print_step "Запуск nginx для получения сертификатов..."
docker-compose up -d nginx
sleep 5

# Получаем сертификат для основного домена
print_step "Получение сертификата для $DOMAIN..."
docker run --rm \
  -v $(pwd)/certificates/nginx/ssl:/etc/letsencrypt \
  -v $(pwd)/certbot/www:/var/www/certbot \
  certbot/certbot certonly --webroot \
  --webroot-path=/var/www/certbot \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN

# Получаем сертификат для тестового домена
print_step "Получение сертификата для $TEST_DOMAIN..."
docker run --rm \
  -v $(pwd)/certificates/nginx/ssl:/etc/letsencrypt \
  -v $(pwd)/certbot/www:/var/www/certbot \
  certbot/certbot certonly --webroot \
  --webroot-path=/var/www/certbot \
  -d $TEST_DOMAIN

print_success "Сертификаты получены"

# Перезапускаем nginx с SSL
print_step "Перезапуск nginx с SSL..."
docker-compose restart nginx

print_success "SSL настройка завершена!"