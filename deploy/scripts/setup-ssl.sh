#!/bin/bash
set -e

cd /opt/pigeongram/deploy

# Загружаем переменные
source config/.env.production

echo "▶ 1. Создание директорий"
mkdir -p certbot/www/.well-known/acme-challenge
mkdir -p certificates/nginx/ssl

echo "▶ 2. Временный HTTP конфиг для получения сертификата"
cp config/nginx/nginx.http.conf config/nginx/nginx.conf

echo "▶ 3. Запуск nginx в HTTP режиме"
docker-compose up -d nginx
sleep 5

echo "▶ 4. Получение сертификата для $DOMAIN"
docker run --rm \
  -v $(pwd)/certificates/nginx/ssl:/etc/letsencrypt \
  -v $(pwd)/certbot/www:/var/www/certbot \
  certbot/certbot certonly --webroot \
  --webroot-path=/var/www/certbot \
  --email "$SSL_EMAIL" \
  --agree-tos \
  --non-interactive \
  --force-renewal \
  -d "$DOMAIN"

echo "▶ 5. Переключение на SSL конфиг"
cp config/nginx/nginx.conf.ssl config/nginx/nginx.conf

echo "▶ 6. Перезапуск nginx с SSL"
docker-compose restart nginx

echo "✅ SSL готов для $DOMAIN"