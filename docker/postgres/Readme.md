# Перейти в папку с Docker-файлами
cd docker/postgres

# Дать права на выполнение скрипта
chmod +x manage.sh

# Запустить контейнеры
./manage.sh start

# Проверить статус
./manage.sh status

# Посмотреть логи
./manage.sh logs

# Подключиться к PostgreSQL
./manage.sh connect

# Выйти из PostgreSQL
\q



# Создать бэкап базы данных
./manage.sh backup

# Восстановить из бэкапа
./manage.sh restore backups/pigeongram_20240313_120000.sql

# Остановить контейнеры (данные сохраняются)
./manage.sh stop

# Полная очистка (удаляет все данные!)
./manage.sh clean

# Зайти в pgAdmin
# Открыть браузер: http://localhost:5050
# Email: admin@pigeongram.local
# Пароль: admin

🎯 Особенности этой конфигурации

    Постоянное хранение - данные сохраняются в Docker volume

    Автоматическая инициализация - таблицы и тестовые данные создаются при первом запуске

    Резервное копирование - встроенные скрипты для бэкапов

    Мониторинг - pgAdmin для управления БД через веб-интерфейс

    Redis - подготовлен для второго этапа

    Healthcheck - автоматическая проверка готовности БД

    Лимиты ресурсов - контролируемое потребление памяти и CPU
    
    
# Подключиться к PostgreSQL
cd docker/postgres
./manage.sh connect

# Проверить данные
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM messages;

# Выйти
\q
