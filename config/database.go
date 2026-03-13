package config

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pigeongram/models"
)

// DatabaseConfig хранит настройки подключения к БД
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewDefaultConfig создает конфигурацию по умолчанию для разработки
func NewDefaultConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "pigeongram",
		Password: "pigeongram_secret",
		DBName:   "pigeongram",
		SSLMode:  "disable",
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

	// Подключение к БД
	db, err := gorm.Open(postgres.Open(config.DSN()), &gorm.Config{
		Logger: gormLogger,
		// Отключаем автоматическое создание внешних ключей
		DisableForeignKeyConstraintWhenMigrating: true,
		// Отключаем автоматическое экранирование имен
		NamingStrategy: nil,
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

	// Временно удаляем всё, что может мешать миграции
	log.Println("🔄 Очистка базы данных перед миграцией...")

	// Удаляем представления
	db.Exec("DROP VIEW IF EXISTS active_users CASCADE")
	db.Exec("DROP FUNCTION IF EXISTS get_recent_messages(INTEGER) CASCADE")

	// Удаляем индексы, если они есть (игнорируем ошибки)
	db.Exec("DROP INDEX IF EXISTS idx_users_username")
	db.Exec("DROP INDEX IF EXISTS idx_users_last_seen")
	db.Exec("DROP INDEX IF EXISTS idx_messages_timestamp")
	db.Exec("DROP INDEX IF EXISTS idx_messages_user_id")
	db.Exec("DROP INDEX IF EXISTS idx_sessions_session_id")
	db.Exec("DROP INDEX IF EXISTS idx_sessions_expires_at")

	// Удаляем таблицы в правильном порядке (из-за зависимостей)
	log.Println("🔄 Удаление существующих таблиц...")
	db.Exec("DROP TABLE IF EXISTS sessions CASCADE")
	db.Exec("DROP TABLE IF EXISTS messages CASCADE")
	db.Exec("DROP TABLE IF EXISTS users CASCADE")

	// Теперь создаем таблицы через AutoMigrate
	log.Println("📦 Создание таблиц через AutoMigrate...")

	// Создаем таблицы по одной
	if err := db.AutoMigrate(&models.User{}); err != nil {
		return nil, fmt.Errorf("ошибка миграции users: %w", err)
	}
	log.Println("✅ Таблица users создана")

	if err := db.AutoMigrate(&models.Message{}); err != nil {
		return nil, fmt.Errorf("ошибка миграции messages: %w", err)
	}
	log.Println("✅ Таблица messages создана")

	if err := db.AutoMigrate(&models.Session{}); err != nil {
		return nil, fmt.Errorf("ошибка миграции sessions: %w", err)
	}
	log.Println("✅ Таблица sessions создана")

	// Создаем индексы вручную
	log.Println("📊 Создание индексов...")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)")

	// Восстанавливаем функцию get_recent_messages
	log.Println("🔄 Создание функции get_recent_messages...")
	err = db.Exec(`
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
    `).Error
	if err != nil {
		log.Printf("⚠️ Не удалось создать функцию: %v", err)
	}

	// Восстанавливаем представление active_users
	log.Println("🔄 Создание представления active_users...")
	err = db.Exec(`
        CREATE OR REPLACE VIEW active_users AS
        SELECT DISTINCT u.id, u.username, u.last_seen
        FROM users u
        WHERE u.last_seen > NOW() - INTERVAL '5 minutes';
    `).Error
	if err != nil {
		log.Printf("⚠️ Не удалось создать представление: %v", err)
	}

	// Создаем тестовых пользователей, если их нет
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		log.Println("👤 Создание тестовых пользователей...")
		createTestUsers(db)
	}

	log.Println("✅ База данных успешно инициализирована")
	return db, nil
}

// createTestUsers создает тестовых пользователей
func createTestUsers(db *gorm.DB) {
	testUsers := []models.User{
		{Username: "test", Password: "test"},
		{Username: "admin", Password: "admin"},
	}

	for _, user := range testUsers {
		// В реальном проекте пароль нужно хешировать!
		result := db.Create(&user)
		if result.Error != nil {
			log.Printf("❌ Ошибка создания тестового пользователя %s: %v", user.Username, result.Error)
		} else {
			log.Printf("✅ Создан тестовый пользователь: %s", user.Username)
		}
	}

	// Создаем тестовые сообщения
	var testUser models.User
	var adminUser models.User

	db.Where("username = ?", "test").First(&testUser)
	db.Where("username = ?", "admin").First(&adminUser)

	if testUser.ID != 0 && adminUser.ID != 0 {
		testMessages := []models.Message{
			{UserID: testUser.ID, Content: "Добро пожаловать в PigeonGram! 🕊️", Timestamp: time.Now().Add(-5 * time.Minute)},
			{UserID: adminUser.ID, Content: "База данных PostgreSQL готова к работе!", Timestamp: time.Now().Add(-4 * time.Minute)},
			{UserID: testUser.ID, Content: "Теперь сообщения сохраняются надежно", Timestamp: time.Now().Add(-3 * time.Minute)},
			{UserID: adminUser.ID, Content: "Docker контейнер работает отлично", Timestamp: time.Now().Add(-2 * time.Minute)},
		}

		for _, msg := range testMessages {
			db.Create(&msg)
		}
		log.Printf("✅ Создано %d тестовых сообщений", len(testMessages))
	}
}
