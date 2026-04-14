🎮 Как использовать
1. Без сброса (обычный запуск)
bash

# Данные сохраняются
go run main.go

2. Со сбросом через флаг командной строки
bash

# Полный сброс базы данных
go run main.go -reset-db

# С подтверждением (спросит "Продолжить?")

3. Со сбросом через переменную окружения
bash

# Для Linux/macOS
export PIGEONGRAM_RESET_DB=true
go run main.go

# Для Windows (CMD)
set PIGEONGRAM_RESET_DB=true
go run main.go

# Для Windows (PowerShell)
$env:PIGEONGRAM_RESET_DB = "true"
go run main.go

4. В Docker с переменной окружения
bash

# В docker-compose.yml можно добавить:
environment:
  - PIGEONGRAM_RESET_DB=true

# Или при запуске:
docker run -e PIGEONGRAM_RESET_DB=true -p 8080:8080 pigeongram


🔧 Добавим Makefile цели для удобства

Обновленный Makefile:
makefile

.PHONY: run run-reset run-dev

# Обычный запуск
run:
	go run main.go

# Запуск со сбросом БД
run-reset:
	go run main.go -reset-db

# Запуск в режиме разработки (с авто-перезагрузкой)
dev:
	air -c .air.toml

# Запуск с определенными переменными окружения
run-prod:
	DB_HOST=localhost DB_USER=postgres DB_PASSWORD=secret go run main.go
	

🎯 Примеры использования
bash

# Разработка - данные сохраняются
make run

# Тестирование - начать с чистой БД
make run-reset

# В CI/CD - автоматический сброс
export PIGEONGRAM_RESET_DB=true
go test ./...

# Деплой - без сброса
go run main.go


