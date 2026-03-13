-- Создаем расширение для UUID (понадобится позже)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =====================================================
-- Таблица пользователей
-- =====================================================
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,  -- Будет хранить хеш bcrypt
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Индексы для быстрого поиска
    CONSTRAINT users_username_unique UNIQUE (username)
);

-- Индекс для поиска по username
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_last_seen ON users(last_seen);

-- Комментарии к таблице
COMMENT ON TABLE users IS 'Пользователи PigeonGram';
COMMENT ON COLUMN users.username IS 'Уникальное имя пользователя';
COMMENT ON COLUMN users.password IS 'Хеш пароля (bcrypt)';

-- =====================================================
-- Таблица сообщений
-- =====================================================
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Проверка, что сообщение не пустое
    CONSTRAINT messages_content_not_empty CHECK (length(trim(content)) > 0)
);

-- Индексы для быстрой выборки сообщений
CREATE INDEX idx_messages_timestamp ON messages(timestamp DESC);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);

-- Комментарии
COMMENT ON TABLE messages IS 'Сообщения чата';
COMMENT ON COLUMN messages.user_id IS 'ID автора сообщения';
COMMENT ON COLUMN messages.content IS 'Текст сообщения';

-- =====================================================
-- Таблица сессий
-- =====================================================
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(32) UNIQUE NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Индексы для сессий
CREATE INDEX idx_sessions_session_id ON sessions(session_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- Комментарии
COMMENT ON TABLE sessions IS 'Активные сессии пользователей';
COMMENT ON COLUMN sessions.session_id IS 'ID сессии (из cookie)';
COMMENT ON COLUMN sessions.expires_at IS 'Время истечения сессии';

-- =====================================================
-- Добавляем тестовых пользователей
-- =====================================================
-- Пароли пока в открытом виде, позже заменим на хеши
INSERT INTO users (username, password) VALUES
    ('test', 'test'),
    ('admin', 'admin')
ON CONFLICT (username) DO NOTHING;

-- =====================================================
-- Добавляем тестовые сообщения
-- =====================================================
DO $$
DECLARE
    test_user_id INTEGER;
    admin_user_id INTEGER;
BEGIN
    -- Получаем ID пользователей
    SELECT id INTO test_user_id FROM users WHERE username = 'test';
    SELECT id INTO admin_user_id FROM users WHERE username = 'admin';
    
    -- Добавляем сообщения только если их еще нет
    IF NOT EXISTS (SELECT 1 FROM messages LIMIT 1) THEN
        INSERT INTO messages (user_id, content, timestamp) VALUES
            (test_user_id, 'Добро пожаловать в PigeonGram! 🕊️', NOW() - INTERVAL '5 minutes'),
            (admin_user_id, 'База данных PostgreSQL готова к работе!', NOW() - INTERVAL '4 minutes'),
            (test_user_id, 'Теперь сообщения сохраняются надежно', NOW() - INTERVAL '3 minutes'),
            (admin_user_id, 'Docker контейнер работает отлично', NOW() - INTERVAL '2 minutes'),
            (test_user_id, 'Можно начинать разработку!', NOW() - INTERVAL '1 minute');
    END IF;
END $$;

-- =====================================================
-- Создаем функцию для очистки старых сессий
-- =====================================================
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM sessions WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- Создаем функцию для получения последних сообщений
-- =====================================================
CREATE OR REPLACE FUNCTION get_recent_messages(limit_count INTEGER)
RETURNS TABLE (
    message_id INTEGER,
    username VARCHAR,
    content TEXT,
    message_time TIMESTAMP WITH TIME ZONE
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        m.id,
        u.username,
        m.content,
        m.timestamp
    FROM messages m
    JOIN users u ON m.user_id = u.id
    WHERE m.deleted_at IS NULL
    ORDER BY m.timestamp DESC
    LIMIT limit_count;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- Настройки производительности
-- =====================================================
-- Включаем автовакуум для автоматической очистки
ALTER SYSTEM SET autovacuum = on;
ALTER SYSTEM SET autovacuum_vacuum_scale_factor = 0.1;
ALTER SYSTEM SET autovacuum_analyze_scale_factor = 0.05;

-- Настройки памяти (адаптируйте под свой сервер)
ALTER SYSTEM SET shared_buffers = '128MB';
ALTER SYSTEM SET work_mem = '4MB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';

-- Применяем настройки
SELECT pg_reload_conf();

-- =====================================================
-- Создаем пользователя для приложения (если нужно)
-- =====================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_user WHERE usename = 'pigeongram_app') THEN
        CREATE USER pigeongram_app WITH PASSWORD 'app_password';
        GRANT CONNECT ON DATABASE pigeongram TO pigeongram_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO pigeongram_app;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO pigeongram_app;
    END IF;
END $$;

-- =====================================================
-- Создаем представление для активных пользователей
-- =====================================================
CREATE OR REPLACE VIEW active_users AS
SELECT DISTINCT u.id, u.username, u.last_seen
FROM users u
WHERE u.last_seen > NOW() - INTERVAL '5 minutes';

-- =====================================================
-- Статистика базы данных
-- =====================================================
COMMENT ON DATABASE pigeongram IS 'База данных мессенджера PigeonGram';

-- Выводим информацию о созданных таблицах
SELECT '✅ База данных PigeonGram успешно инициализирована' as message;
SELECT '📊 Таблицы:' as info;
SELECT tablename, tableowner, pg_size_pretty(pg_total_relation_size(tablename::regclass)) as size
FROM pg_tables 
WHERE schemaname = 'public';