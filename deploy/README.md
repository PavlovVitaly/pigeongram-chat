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

🆘 Troubleshooting
# Проверка статуса
pigeongram status

# Просмотр логов при ошибках
pigeongram logs app 100
pigeongram logs postgres 100

# Проверка доступности БД
docker exec -it pigeongram_postgres psql -U pigeongram -d pigeongram -c "SELECT 1"

# Проверка метрик
curl http://localhost:8080/metrics | head -20

# Ручной запуск конкретного сервиса
cd /opt/pigeongram/repo/docker/postgres && ./manage.sh start
cd /opt/pigeongram/repo/docker/monitoring && ./monitor.sh start

📦 Резервное копирование

Бэкапы автоматически создаются в /opt/pigeongram/backups/ и хранятся 30 дней.
bash

# Создать бэкап вручную
pigeongram backup

# Восстановить из бэкапа
pigeongram restore 20250317_143022

🔄 Обновление
bash

# Ручное обновление
pigeongram update

# Автоматическое обновление настроено на 3:00 каждую ночь

📁 Структура директорий
text

/opt/pigeongram/
├── config/              # Конфигурационные файлы
├── backups/             # Резервные копии
├── logs/                # Логи приложения
├── ssl/                  # SSL сертификаты
├── scripts/             # Скрипты управления
├── repo/                 # Код приложения
│   ├── docker/           # Docker файлы
│   │   ├── postgres/     # PostgreSQL + MinIO
│   │   └── monitoring/   # Prometheus + Grafana
│   └── ...               # Остальной код
└── data/                 # Данные Docker контейнеров
    ├── postgres/
    ├── redis/
    ├── minio/
    ├── prometheus/
    └── grafana/