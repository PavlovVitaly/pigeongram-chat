#!/bin/bash
# =====================================================
# PigeonGram - Настройка SSL сертификатов (Let's Encrypt)
# =====================================================

set -e

# =============================================
# ИЗМЕНЕНИЕ: домены из .env.production
# =============================================
DOMAIN=${DOMAIN:-"test.pigeongram.com.ru"}
TEST_DOMAIN=${TEST_DOMAIN:-"test.pigeongram.com.ru"}
EMAIL=${SSL_EMAIL:-"admin@test.pigeongram.com.ru"}
WEBROOT="/var/www/certbot"

cd /opt/pigeongram/deploy

echo "▶ Создание директорий для certbot"
mkdir -p certbot/www/.well-known/acme-challenge
chmod 755 certbot/www
chmod 755 certbot/www/.well-known
chmod 755 certbot/www/.well-known/acme-challenge

echo "▶ Получение сертификата для $DOMAIN"
docker run --rm \
  -v $(pwd)/certificates/nginx/ssl:/etc/letsencrypt \
  -v $(pwd)/certbot/www:/var/www/certbot \
  certbot/certbot certonly --webroot \
  --webroot-path=$WEBROOT \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN

if [ ! -z "$TEST_DOMAIN" ]; then
    echo "▶ Получение сертификата для $TEST_DOMAIN"
    docker run --rm \
      -v $(pwd)/certificates/nginx/ssl:/etc/letsencrypt \
      -v $(pwd)/certbot/www:/var/www/certbot \
      certbot/certbot certonly --webroot \
      --webroot-path=$WEBROOT \
      -d $TEST_DOMAIN
fi

echo "✅ SSL сертификаты получены"