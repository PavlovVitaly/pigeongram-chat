#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🐳 PigeonGram Docker Management${NC}"
echo "================================"

case "$1" in
  start)
    echo -e "${GREEN}▶️ Запуск контейнеров...${NC}"
    docker-compose up -d
    echo -e "${GREEN}✅ Готово!${NC}"
    echo -e "PostgreSQL: ${BLUE}localhost:5432${NC}"
    echo -e "pgAdmin:    ${BLUE}http://localhost:5050${NC}"
    echo -e "Redis:      ${BLUE}localhost:6379${NC}"
    ;;
    
  stop)
    echo -e "${YELLOW}⏹️ Остановка контейнеров...${NC}"
    docker-compose down
    echo -e "${GREEN}✅ Контейнеры остановлены${NC}"
    ;;
    
  restart)
    echo -e "${YELLOW}🔄 Перезапуск...${NC}"
    docker-compose restart
    echo -e "${GREEN}✅ Готово${NC}"
    ;;
    
  status)
    echo -e "${BLUE}📊 Статус контейнеров:${NC}"
    docker-compose ps
    ;;
    
  logs)
    echo -e "${BLUE}📋 Логи PostgreSQL:${NC}"
    docker-compose logs --tail=100 -f postgres
    ;;
    
  backup)
    BACKUP_FILE="backups/pigeongram_$(date +%Y%m%d_%H%M%S).sql"
    echo -e "${BLUE}💾 Создание бэкапа: $BACKUP_FILE${NC}"
    docker-compose exec postgres pg_dump -U pigeongram pigeongram > $BACKUP_FILE
    echo -e "${GREEN}✅ Бэкап создан: $BACKUP_FILE${NC}"
    ;;
    
  restore)
    if [ -z "$2" ]; then
      echo -e "${RED}❌ Укажите файл для восстановления${NC}"
      echo "Использование: $0 restore backups/backup.sql"
      exit 1
    fi
    echo -e "${YELLOW}⚠️ Восстановление из $2...${NC}"
    cat $2 | docker-compose exec -T postgres psql -U pigeongram -d pigeongram
    echo -e "${GREEN}✅ Восстановление завершено${NC}"
    ;;
    
  connect)
    echo -e "${BLUE}🔌 Подключение к PostgreSQL...${NC}"
    docker-compose exec postgres psql -U pigeongram -d pigeongram
    ;;
    
  clean)
    echo -e "${RED}⚠️  Очистка всех данных...${NC}"
    docker-compose down -v
    echo -e "${GREEN}✅ Все данные удалены${NC}"
    ;;
    
  *)
    echo -e "Использование: $0 {start|stop|restart|status|logs|backup|restore|connect|clean}"
    echo ""
    echo "Команды:"
    echo "  start   - Запустить контейнеры"
    echo "  stop    - Остановить контейнеры"
    echo "  restart - Перезапустить контейнеры"
    echo "  status  - Показать статус"
    echo "  logs    - Показать логи PostgreSQL"
    echo "  backup  - Создать бэкап базы данных"
    echo "  restore - Восстановить из бэкапа"
    echo "  connect - Подключиться к PostgreSQL"
    echo "  clean   - Очистить все данные (осторожно!)"
    exit 1
    ;;
esac