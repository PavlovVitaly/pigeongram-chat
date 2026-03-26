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