#!/bin/bash

PROJECT_ROOT="/opt/pigeongram"
BACKUP_DIR="$PROJECT_ROOT/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

print_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${PURPLE}ℹ️ $1${NC}"
}

# Создаем директорию для бэкапов
mkdir -p $BACKUP_DIR

print_step "Начало резервного копирования PigeonGram"

# Бэкап PostgreSQL через manage.sh
print_info "Бэкап PostgreSQL..."
cd $PROJECT_ROOT/repo/docker/postgres
./manage.sh backup
cp backups/*.sql $BACKUP_DIR/postgres_$DATE.sql 2>/dev/null || true
print_success "PostgreSQL сохранен"

# Бэкап Redis
print_info "Бэкап Redis..."
if docker ps | grep -q pigeongram_redis; then
    docker exec pigeongram_redis redis-cli -a redis_secret SAVE
    docker cp pigeongram_redis:/data/dump.rdb $BACKUP_DIR/redis_$DATE.rdb 2>/dev/null || true
    print_success "Redis сохранен"
else
    print_warning "Redis не запущен, пропускаем"
fi

# Бэкап MinIO
print_info "Бэкап MinIO..."
if docker ps | grep -q pigeongram_minio; then
    docker exec pigeongram_minio mc ls myminio > $BACKUP_DIR/minio_$DATE.txt 2>/dev/null || true
    print_success "MinIO метаданные сохранены"
fi

# Бэкап конфигурации
print_info "Бэкап конфигурации..."
tar -czf $BACKUP_DIR/config_$DATE.tar.gz -C $PROJECT_ROOT config/ .env* 2>/dev/null || true
print_success "Конфигурация сохранена"

# Бэкап логов
print_info "Бэкап логов..."
if [ -d "$PROJECT_ROOT/logs" ]; then
    tar -czf $BACKUP_DIR/logs_$DATE.tar.gz -C $PROJECT_ROOT logs/ 2>/dev/null || true
    print_success "Логи сохранены"
fi

# Бэкап Prometheus данных
print_info "Бэкап Prometheus..."
if docker volume ls | grep -q pigeongram_prometheus_data; then
    docker run --rm -v pigeongram_prometheus_data:/data -v $BACKUP_DIR:/backup alpine \
        tar -czf /backup/prometheus_$DATE.tar.gz -C /data . 2>/dev/null || true
    print_success "Prometheus данные сохранены"
fi

# Удаляем старые бэкапы (старше 30 дней)
print_info "Очистка старых бэкапов..."
find $BACKUP_DIR -name "*.sql" -type f -mtime +30 -delete
find $BACKUP_DIR -name "*.rdb" -type f -mtime +30 -delete
find $BACKUP_DIR -name "*.tar.gz" -type f -mtime +30 -delete
find $BACKUP_DIR -name "*.txt" -type f -mtime +30 -delete
print_success "Старые бэкапы удалены"

echo ""
echo "📦 Бэкап завершен: $BACKUP_DIR/backup_$DATE"
echo "   Размер: $(du -sh $BACKUP_DIR | cut -f1)"
ls -lh $BACKUP_DIR | grep "$DATE"