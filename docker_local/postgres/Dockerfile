# Используем официальный образ PostgreSQL
FROM postgres:15-alpine

# Устанавливаем локаль для правильной сортировки
RUN apk add --no-cache tzdata

# Копируем скрипт инициализации
COPY init.sql /docker-entrypoint-initdb.d/

# Устанавливаем переменные окружения по умолчанию (можно переопределить)
ENV POSTGRES_USER=pigeongram
ENV POSTGRES_PASSWORD=pigeongram_secret
ENV POSTGRES_DB=pigeongram
ENV PGDATA=/var/lib/postgresql/data/pgdata

# Открываем порт PostgreSQL
EXPOSE 5432

# Хелсчек для проверки готовности БД
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD pg_isready -U $POSTGRES_USER -d $POSTGRES_DB || exit 1

# Метка с версией
LABEL version="1.0" \
      description="PostgreSQL for PigeonGram" \
      maintainer="PigeonGram Team"