package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"

	"pigeongram/config"
	"pigeongram/internal/storage"
	"pigeongram/internal/websocket"
	"pigeongram/repository/cache"
	"pigeongram/repository/postgres"
	"pigeongram/web"
)

func main() {
	// Флаги командной строки
	resetDB := flag.Bool("reset-db", false, "Сбросить базу данных при запуске")
	useRedis := flag.Bool("use-redis", true, "Использовать Redis для кэша и Pub/Sub")
	port := flag.Int("port", 8080, "Порт сервера")
	serverID := flag.String("server-id", "", "ID сервера (если не указан, генерируется)")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	// Генерируем ID сервера если не указан
	if *serverID == "" {
		hostname, _ := os.Hostname()
		*serverID = fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	}

	log.Printf("🚀 Запуск сервера %s на порту %d", *serverID, *port)
	log.Printf("📊 Режим: resetDB=%v, useRedis=%v", *resetDB, *useRedis)

	// Инициализация конфигурации БД
	dbConfig := config.NewDefaultConfig()
	if *resetDB {
		dbConfig.ResetDB = true
		log.Println("⚠️ Режим сброса базы данных активирован")
	}

	// Инициализация PostgreSQL
	log.Println("📦 Подключение к PostgreSQL...")
	db, err := config.InitPostgres(dbConfig)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	log.Println("✅ PostgreSQL подключен успешно")

	// Инициализация Redis клиента
	var redisClient *redis.Client
	redisConfig := config.NewRedisConfigReader()

	if *useRedis && redisConfig.IsEnabled() {
		host, port, password, db, _ := redisConfig.Load()

		redisClient = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", host, port),
			Password:     password,
			DB:           db,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 2,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("⚠️ Ошибка подключения к Redis: %v", err)
			log.Println("⚠️ Redis будет отключен")
			redisClient = nil
		} else {
			log.Println("✅ Redis подключен успешно")
		}
	} else {
		log.Println("⚠️ Redis отключен (используется MemoryCache)")
	}

	// Инициализация кэша
	var messageCache cache.Cache
	if redisClient != nil {
		host, port, password, db, ttl := redisConfig.Load()
		redisCache, err := cache.NewRedisCache(cache.RedisConfig{
			Host:     host,
			Port:     port,
			Password: password,
			DB:       db,
			TTL:      ttl,
			Prefix:   redisConfig.GetCachePrefix(),
		})
		if err == nil {
			messageCache = redisCache
			log.Println("✅ Redis кэш инициализирован")
		} else {
			log.Printf("⚠️ Ошибка инициализации Redis кэша: %v", err)
		}
	}

	if messageCache == nil {
		messageCache = cache.NewMemoryCache(5 * time.Minute)
		log.Println("⚠️ Используется MemoryCache (в памяти)")
	}

	// Создаем репозитории
	userRepo := postgres.NewUserRepository(db)
	msgRepo := postgres.NewMessageRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)

	// Создаем WebSocket менеджер
	wsManager := websocket.NewManager(
		msgRepo,
		userRepo,
		messageCache,
		redisClient,
		*serverID,
	)

	// Регистрируем сервер в Redis (для кластера)
	if redisClient != nil {
		registry := websocket.NewServerRegistry(redisClient, *serverID)
		ctx := context.Background()
		if err := registry.Register(ctx); err != nil {
			log.Printf("⚠️ Ошибка регистрации сервера: %v", err)
		} else {
			log.Println("✅ Сервер зарегистрирован в Redis кластере")
		}
		defer func() {
			ctx := context.Background()
			if err := registry.Unregister(ctx); err != nil {
				log.Printf("⚠️ Ошибка при дерегистрации сервера: %v", err)
			}
		}()
	}

	// Запускаем WebSocket менеджер
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wsManager.Run(ctx)

	// Инициализируем хранилища в web пакете
	web.InitStores(userRepo, msgRepo, sessionRepo, messageCache)
	web.InitWebSocket(wsManager)

	// Инициализация MinIO
	log.Println("📦 Подключение к MinIO...")
	minioConfig := config.NewMinIOConfig()
	minioClient, err := storage.NewMinIOClient(minioConfig)
	if err != nil {
		log.Printf("⚠️ Ошибка подключения к MinIO: %v", err)
		log.Println("⚠️ Функции файлов будут недоступны")
		minioClient = nil
	} else {
		log.Println("✅ MinIO подключен успешно")
	}

	// Создаем file handler с wsManager
	fileHandler := web.NewFileHandler(minioClient, wsManager)

	// Маршруты
	http.HandleFunc("/", web.LoginPage)
	http.HandleFunc("/login", web.LoginHandler)
	http.HandleFunc("/register", web.RegisterPage)
	http.HandleFunc("/register-handler", web.RegisterHandler)
	http.HandleFunc("/chat", web.AuthMiddleware(web.ChatPage))
	http.HandleFunc("/logout", web.LogoutHandler)
	http.HandleFunc("/ws", web.AuthMiddleware(web.WebSocketHandler))

	if minioClient != nil {
		http.HandleFunc("/files", fileHandler.FilePage)
		http.HandleFunc("/api/files/upload-url", fileHandler.RequestUpload)
		http.HandleFunc("/api/files/upload-complete", fileHandler.UploadComplete)
		http.HandleFunc("/api/files/list", fileHandler.ListFiles)
		http.HandleFunc("/api/files/download-url", fileHandler.GetDownloadURL)
		http.HandleFunc("/api/files/delete", fileHandler.DeleteFile)
		http.HandleFunc("/api/files/test-notify", fileHandler.TestFileNotification) // для отладки

		// Debug маршруты (только для разработки)
		if config.GetServerEnvironment() == "development" {
			http.HandleFunc("/debug/test-notify", fileHandler.TestFileNotification)
			http.HandleFunc("/debug/websocket", fileHandler.DebugWebSocket)
			http.HandleFunc("/debug/send-to-user", fileHandler.DebugSendTestEvent)
			http.HandleFunc("/debug/check-file", fileHandler.DebugCheckFile)
			log.Println("🔧 Debug маршруты активированы:")
			log.Println("   GET /debug/test-notify?chat_id=general - отправить тестовое уведомление всем")
			log.Println("   GET /debug/websocket - информация о WebSocket")
			log.Println("   GET /debug/send-to-user?user=test&chat_id=general - отправить уведомление конкретному пользователю")
		}
		log.Println("📁 Файловое хранилище активировано с WebSocket уведомлениями")
	}

	if config.GetServerEnvironment() == "development" {
		http.HandleFunc("/debug/stats", web.DebugStatsHandler)
		http.HandleFunc("/health", web.HealthCheckHandler)
		http.HandleFunc("/metrics", web.MetricsHandler)
	}

	// Статические файлы
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Запускаем сервер
	serverAddr := fmt.Sprintf(":%d", *port)
	log.Printf("🕊️ PigeonGram сервер %s запущен на http://localhost%s", *serverID, serverAddr)
	log.Printf("📊 Статистика: http://localhost%s/debug/stats", serverAddr)

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatal("❌ Ошибка запуска сервера:", err)
	}
}
