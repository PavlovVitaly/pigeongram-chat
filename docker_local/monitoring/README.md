# Запустить мониторинг
./monitor.sh start

# Остановить мониторинг
./monitor.sh stop

# Перезапустить мониторинг
./monitor.sh restart

# Посмотреть статус
./monitor.sh status

# Посмотреть логи Prometheus
./monitor.sh logs prometheus

# Посмотреть логи Grafana (последние 100 строк)
./monitor.sh logs grafana 100

# Проверить здоровье системы
./monitor.sh health

# Обновить токен MinIO вручную
./monitor.sh token

# Показать статистику
./monitor.sh stats

# Очистить все данные (осторожно!)
./monitor.sh clean

# Показать интерактивное меню
./monitor.sh menu

# Показать справку
./monitor.sh help