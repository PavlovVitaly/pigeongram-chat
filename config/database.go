package config

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseConfig хранит настройки подключения к БД
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	ResetDB  bool
	Force    bool
}

// NewDefaultConfig создает конфигурацию по умолчанию для разработки
func NewDefaultConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     GetEnv("DB_HOST", "localhost"),
		Port:     GetEnvAsInt("DB_PORT", 5432),
		User:     GetEnv("DB_USER", "pigeongram"),
		Password: GetEnv("DB_PASSWORD", "pigeongram_secret"),
		DBName:   GetEnv("DB_NAME", "pigeongram"),
		SSLMode:  GetEnv("DB_SSLMODE", "disable"),
		ResetDB:  GetEnvAsBool("PIGEONGRAM_RESET_DB", false),
		Force:    GetEnvAsBool("PIGEONGRAM_FORCE", false),
	}
}

// DSN возвращает строку подключения
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// InitPostgres инициализирует подключение к PostgreSQL
func InitPostgres(config *DatabaseConfig) (*gorm.DB, error) {
	// Настройка логгера GORM
	gormLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// Подключение к БД с отключенными внешними ключами
	db, err := gorm.Open(postgres.Open(config.DSN()), &gorm.Config{
		Logger:                                   gormLogger,
		DisableForeignKeyConstraintWhenMigrating: true, // Отключаем внешние ключи при миграции
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	// Получение сырого SQL DB для настройки пула соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("ошибка получения sql.DB: %w", err)
	}

	// Настройка пула соединений
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Если нужен сброс - удаляем всё
	if config.ResetDB {
		log.Println("⚠️ Сброс базы данных...")

		// Удаляем представления
		db.Exec("DROP VIEW IF EXISTS active_users CASCADE")

		// Удаляем функции
		db.Exec("DROP FUNCTION IF EXISTS get_recent_messages(INTEGER) CASCADE")

		// Удаляем таблицы в правильном порядке
		db.Exec("DROP TABLE IF EXISTS sessions CASCADE")
		db.Exec("DROP TABLE IF EXISTS messages CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")

		log.Println("✅ База данных очищена")
	}

	// Включаем расширение для UUID (если нужно)
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")

	// Создаем таблицы вручную через Exec, чтобы избежать проблем с индексами
	log.Println("📦 Создание таблиц...")

	// Таблица users
	if err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            username VARCHAR(50) NOT NULL,
            password VARCHAR(255) NOT NULL,
            last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            deleted_at TIMESTAMP WITH TIME ZONE,
            CONSTRAINT users_username_unique UNIQUE (username)
        );
    `).Error; err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы users: %w", err)
	}

	// Таблица messages
	if err := db.Exec(`
        CREATE TABLE IF NOT EXISTS messages (
            id SERIAL PRIMARY KEY,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            deleted_at TIMESTAMP WITH TIME ZONE
        );
    `).Error; err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы messages: %w", err)
	}

	// Таблица sessions
	if err := db.Exec(`
        CREATE TABLE IF NOT EXISTS sessions (
            id SERIAL PRIMARY KEY,
            session_id VARCHAR(32) UNIQUE NOT NULL,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            deleted_at TIMESTAMP WITH TIME ZONE
        );
    `).Error; err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы sessions: %w", err)
	}

	log.Println("✅ Таблицы созданы")

	// Создаем индексы
	createIndexes(db)

	// Создаем функции
	createFunctions(db)

	// Создаем представления
	createViews(db)

	return db, nil
}

// createIndexes создает индексы
func createIndexes(db *gorm.DB) {
	log.Println("📊 Создание индексов...")

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)",
		"CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen)",
		"CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)",
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("⚠️ Ошибка создания индекса %s: %v", idx, err)
		}
	}
	log.Println("✅ Индексы созданы")
}

// createFunctions создает полезные функции
func createFunctions(db *gorm.DB) {
	log.Println("🔧 Создание функций...")

	functionSQL := `
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
    $$ LANGUAGE plpgsql;`

	if err := db.Exec(functionSQL).Error; err != nil {
		log.Printf("⚠️ Ошибка создания функции: %v", err)
	}

	log.Println("✅ Функции созданы")
}

// createViews создает представления
func createViews(db *gorm.DB) {
	log.Println("👁️ Создание представлений...")

	viewSQL := `
    CREATE OR REPLACE VIEW active_users AS
    SELECT DISTINCT u.id, u.username, u.last_seen
    FROM users u
    WHERE u.last_seen > NOW() - INTERVAL '5 minutes'
    ORDER BY u.username;`

	if err := db.Exec(viewSQL).Error; err != nil {
		log.Printf("⚠️ Ошибка создания представления: %v", err)
	}

	log.Println("✅ Представления созданы")
}
