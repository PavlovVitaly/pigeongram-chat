#!/bin/bash

echo "🔍 Тестирование PostgreSQL подключения..."

# 1. Проверяем, что PostgreSQL запущен
pg_isready -h localhost -p 5432
if [ $? -ne 0 ]; then
    echo "❌ PostgreSQL не запущен!"
    exit 1
fi

# 2. Проверяем наличие базы данных
psql -U postgres -lqt | cut -d \| -f 1 | grep -qw pigeongram
if [ $? -eq 0 ]; then
    echo "✅ База данных 'pigeongram' существует"
else
    echo "❌ База данных 'pigeongram' не найдена"
    exit 1
fi

# 3. Запускаем приложение в фоне
echo "🚀 Запуск PigeonGram..."
go run main.go &
APP_PID=$!

# 4. Ждем 3 секунды
sleep 3

# 5. Проверяем, что приложение запустилось
ps -p $APP_PID > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ PigeonGram запущен (PID: $APP_PID)"
else
    echo "❌ Ошибка запуска PigeonGram"
    exit 1
fi

# 6. Тестируем API
echo "📡 Тестирование HTTP endpoints..."
curl -s http://localhost:8080 > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ HTTP сервер отвечает"
else
    echo "❌ HTTP сервер не отвечает"
    kill $APP_PID
    exit 1
fi

# 7. Проверяем таблицы в БД
echo "📊 Проверка таблиц..."
psql -U postgres -d pigeongram -c "\dt" | grep -E "users|messages|sessions"
if [ $? -eq 0 ]; then
    echo "✅ Таблицы созданы"
else
    echo "❌ Таблицы не найдены"
fi

# 8. Останавливаем приложение
echo "🛑 Остановка PigeonGram..."
kill $APP_PID

echo "✨ Тестирование завершено!"