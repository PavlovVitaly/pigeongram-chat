# 1. Сделать бэкап БД
docker exec pigeongram_postgres pg_dump -U pigeongram pigeongram > backup.sql

# 2. Остановить приложение (но оставить БД)
cd /opt/pigeongram/deploy
docker-compose stop app

# 3. Обновить код
cd /opt/pigeongram/app
git pull
go mod download

# 4. Пересобрать образ
docker build -t pigeongram:latest .

# 5. Запустить миграцию паролей
docker run --rm --network deploy_pigeongram_network \
  -e DB_DSN="host=postgres port=5432 user=pigeongram password=pigeongram123 dbname=pigeongram sslmode=disable" \
  pigeongram:latest go run scripts/migrate-passwords.go

# 6. Запустить новую версию
cd /opt/pigeongram/deploy
docker-compose up -d app

# 7. Проверить логи
docker-compose logs app

# Проверить что пароль хешируется
docker exec pigeongram_postgres psql -U pigeongram -c "SELECT username, password FROM users LIMIT 1;"

# Должен быть хеш вида: $2a$10$...