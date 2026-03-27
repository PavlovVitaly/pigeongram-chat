/opt/pigeongram/
├── docker-compose.yaml
├── .env.production
├── init.sh
├── init-scripts/
│   └── init-minio.sh
├── logs/
└── repo/
    ├── Dockerfile
    ├── go.mod
    ├── internal/
    │   └── storage/
    │       └── minio.go (с регионом)
    └── ... (остальной код)


cd /opt/pigeongram
chmod +x init.sh
./init.sh


📋 Что автоматизировано
Шаг	Автоматизация
Сборка приложения	docker build
Запуск PostgreSQL	docker-compose up -d postgres
Запуск Redis	docker-compose up -d redis
Запуск MinIO	docker-compose up -d minio
Создание bucket	init-minio.sh
Запуск приложения	docker-compose up -d app
Проверка статуса	docker-compose ps
🔐 Пароли и пользователи
Сервис	Пользователь	Пароль	Задается
PostgreSQL	pigeongram	pigeongram123	Переменные окружения
Redis	—	redis123	command: redis-server --requirepass
MinIO	minioadmin	minioadmin	Переменные окружения
MinIO bucket	—	—	init-minio.sh

Теперь достаточно одной команды ./init.sh для полного развертывания


init-server.sh
📋 Что делает скрипт
Шаг	Действие
1	Обновление системы (apt update/upgrade)
2	Установка базовых пакетов (curl, git, vim, htop, и т.д.)
3	Установка Docker
4	Установка Docker Compose
5	Установка Go 1.26
6	Создание пользователя app
7	Настройка фаервола (открыты порты 22,80,443,8080,9000,9001,9090,3000)
8	Настройка fail2ban для защиты SSH
9	Настройка автоматических обновлений безопасности
10	Создание swap файла (2GB)
11	Оптимизация параметров ядра
12	Создание директорий /opt/pigeongram/*
13	Проверка установки
