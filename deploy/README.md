# 🚀 Развертывание PigeonGram на сервере

## Быстрый старт

```bash
# 1. Скопируйте файлы на сервер
scp -r deploy/* root@your-server:/opt/pigeongram/

# 2. Зайдите на сервер
ssh root@your-server

# 3. Запустите инициализацию
cd /opt/pigeongram
chmod +x scripts/*.sh
sudo ./scripts/01-init-server.sh

# 4. Настройте конфигурацию
cp config/.env.production.example config/.env.production
nano config/.env.production

# 5. Запустите деплой (от обычного пользователя)
su - youruser
cd /opt/pigeongram
./scripts/02-deploy-app.sh


# Статус всех сервисов
pigeongram status

# Запустить все
pigeongram start

# Остановить все
pigeongram stop

# Создать полный бэкап
pigeongram backup

# Восстановить из бэкапа
pigeongram restore 20250317_143022

# Обновить приложение
pigeongram update

# Посмотреть логи приложения
pigeongram logs app

# Посмотреть логи PostgreSQL
pigeongram logs postgres

# Посмотреть метрики
pigeongram metrics

# Управление PostgreSQL
pigeongram postgres backup
pigeongram postgres restore backups/backup.sql

# Управление мониторингом
pigeongram monitor health
pigeongram monitor token
pigeongram monitor dashboards

# Интерактивное меню
pigeongram menu