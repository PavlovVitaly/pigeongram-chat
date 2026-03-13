package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
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
	ResetDB  bool // Флаг для сброса базы данных
}

// NewDefaultConfig создает конфигурацию по умолчанию для разработки
func NewDefaultConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvAsInt("DB_PORT", 5432),
		User:     getEnv("DB_USER", "pigeongram"),
		Password: getEnv("DB_PASSWORD", "pigeongram_secret"),
		DBName:   getEnv("DB_NAME", "pigeongram"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
		ResetDB:  getEnvAsBool("PIGEONGRAM_RESET_DB", false), // ← Только ENV, без флагов
	}
}

// Вспомогательные функции для работы с переменными окружения
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
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
		Logger:                                   gormLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
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

	// ===== ОПЦИОНАЛЬНЫЙ СБРОС БАЗЫ ДАННЫХ =====
	if config.ResetDB {
		log.Println("⚠️⚠️⚠️ ВНИМАНИЕ: Выполняется сброс базы данных! ⚠️⚠️⚠️")
		log.Println("Все существующие данные будут удалены!")

		// Спрашиваем подтверждение, если запущено в интерактивном режиме
		if !isRunningInDocker() {
			fmt.Print("Продолжить? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				log.Println("❌ Сброс отменен")
				os.Exit(0)
			}
		}

		// Удаляем всё в правильном порядке
		log.Println("🔄 Удаление существующих таблиц...")
		db.Exec("DROP VIEW IF EXISTS active_users CASCADE")
		db.Exec("DROP FUNCTION IF EXISTS get_recent_messages(INTEGER) CASCADE")
		db.Exec("DROP TABLE IF EXISTS sessions CASCADE")
		db.Exec("DROP TABLE IF EXISTS messages CASCADE")
		db.Exec("DROP TABLE IF EXISTS users CASCADE")

		log.Println("✅ База данных очищена")

		// Создаем таблицы заново
		log.Println("📦 Создание таблиц...")
		if err := db.AutoMigrate(&models.User{}, &models.Message{}, &models.Session{}); err != nil {
			return nil, fmt.Errorf("ошибка миграции: %w", err)
		}

		createIndexes(db)
		createFunctionsAndViews(db)
		createTestData(db)

		log.Println("✅ База данных пересоздана с тестовыми данными")
		return db, nil
	}

	// ===== НОРМАЛЬНЫЙ РЕЖИМ (БЕЗ СБРОСА) =====

	// Проверяем, есть ли таблицы
	var tableCount int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)

	if tableCount == 0 {
		log.Println("📦 Таблицы не найдены. Создаем новые...")

		// Создаем таблицы
		if err := db.AutoMigrate(&models.User{}, &models.Message{}, &models.Session{}); err != nil {
			return nil, fmt.Errorf("ошибка миграции: %w", err)
		}

		createIndexes(db)
		createFunctionsAndViews(db)
		createTestData(db)
	} else {
		log.Printf("✅ Найдено %d таблиц. Проверяем схему...", tableCount)

		// Обновляем схему без потери данных
		if err := db.AutoMigrate(&models.User{}, &models.Message{}, &models.Session{}); err != nil {
			log.Printf("⚠️ Ошибка при обновлении схемы: %v", err)
		}

		// Обновляем индексы, функции и представления
		createIndexes(db)
		createFunctionsAndViews(db)
	}

	return db, nil
}

// isRunningInDocker проверяет, запущено ли приложение в Docker контейнере
func isRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// createIndexes создает индексы
func createIndexes(db *gorm.DB) {
	log.Println("📊 Проверка индексов...")

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
			log.Printf("⚠️ Ошибка создания индекса: %v", err)
		}
	}
	log.Println("✅ Индексы проверены")
}

// createFunctionsAndViews создает функции и представления
func createFunctionsAndViews(db *gorm.DB) {
	log.Println("🔄 Проверка функций и представлений...")

	// Удаляем старые версии
	db.Exec("DROP VIEW IF EXISTS active_users CASCADE")
	db.Exec("DROP FUNCTION IF EXISTS get_recent_messages(INTEGER) CASCADE")

	// Создаем функцию get_recent_messages
	db.Exec(`
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
    `)

	// Создаем представление active_users
	db.Exec(`
        CREATE OR REPLACE VIEW active_users AS
        SELECT DISTINCT u.id, u.username, u.last_seen
        FROM users u
        WHERE u.last_seen > NOW() - INTERVAL '5 minutes';
    `)

	log.Println("✅ Функции и представления проверены")
}

// createTestData создает тестовые данные
func createTestData(db *gorm.DB) {
	log.Println("👤 Проверка наличия тестовых данных...")

	var userCount int64
	db.Model(&models.User{}).Count(&userCount)

	if userCount == 0 {
		log.Println("📝 Создание тестовых пользователей...")

		testUsers := []models.User{
			{Username: "test", Password: "test"},
			{Username: "admin", Password: "admin"},
		}

		for _, user := range testUsers {
			db.Create(&user)
		}

		// Получаем созданных пользователей
		var testUser, adminUser models.User
		db.Where("username = ?", "test").First(&testUser)
		db.Where("username = ?", "admin").First(&adminUser)

		// Создаем тестовые сообщения
		if testUser.ID != 0 && adminUser.ID != 0 {
			log.Println("💬 Создание тестовых сообщений...")

			testMessages := []models.Message{
				{UserID: testUser.ID, Content: "Добро пожаловать в PigeonGram! 🕊️", Timestamp: time.Now().Add(-5 * time.Minute)},
				{UserID: adminUser.ID, Content: "База данных PostgreSQL готова к работе!", Timestamp: time.Now().Add(-4 * time.Minute)},
				{UserID: testUser.ID, Content: "Теперь сообщения сохраняются надежно", Timestamp: time.Now().Add(-3 * time.Minute)},
				{UserID: adminUser.ID, Content: "Данные сохраняются между перезапусками!", Timestamp: time.Now().Add(-2 * time.Minute)},
			}

			for _, msg := range testMessages {
				db.Create(&msg)
			}
			log.Printf("✅ Создано %d тестовых сообщений", len(testMessages))
		}
	} else {
		log.Printf("✅ Найдено %d пользователей", userCount)
	}
}
