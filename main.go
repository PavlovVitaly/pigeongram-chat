package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"pigeongram/web"
	"time"

	"pigeongram/config"
	"pigeongram/repository/cache"
	"pigeongram/repository/postgres"
)

func main() {
	// Флаги командной строки
	resetDB := flag.Bool("reset-db", false, "Сбросить базу данных при запуске")
	flag.Parse()

	// Инициализация генератора случайных чисел
	rand.Seed(time.Now().UnixNano())

	// 1. Инициализация PostgreSQL
	log.Println("📦 Подключение к PostgreSQL...")
	dbConfig := config.NewDefaultConfig()

	// Переопределяем ResetDB из флага командной строки
	if *resetDB {
		dbConfig.ResetDB = true
		log.Println("⚠️⚠️⚠️ РЕЖИМ СБРОСА БАЗЫ ДАННЫХ АКТИВИРОВАН (флаг -reset-db) ⚠️⚠️⚠️")
	} else if dbConfig.ResetDB {
		log.Println("⚠️⚠️⚠️ РЕЖИМ СБРОСА БАЗЫ ДАННЫХ АКТИВИРОВАН (переменная окружения) ⚠️⚠️⚠️")
	}

	db, err := config.InitPostgres(dbConfig)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	log.Println("✅ PostgreSQL подключен успешно")

	// 2. Инициализация кэша (временная реализация в памяти)
	log.Println("🚀 Инициализация кэша...")
	messageCache := cache.NewMemoryCache(5 * time.Minute) // TTL 5 минут

	// 3. Создаем репозитории
	userRepo := postgres.NewUserRepository(db)
	msgRepo := postgres.NewMessageRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)

	// 4. Инициализируем хранилища в web пакете
	web.InitStores(userRepo, msgRepo, sessionRepo, messageCache)

	// 5. Запускаем периодическую очистку сессий
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			ctx := context.Background()
			if err := sessionRepo.Cleanup(ctx); err != nil {
				log.Printf("❌ Ошибка очистки сессий: %v", err)
			} else {
				log.Println("🧹 Очистка истекших сессий выполнена")
			}
		}
	}()

	// 6. Запускаем WebSocket менеджер
	web.InitWebSocket()

	// Настройка маршрутов
	http.HandleFunc("/", web.LoginPage)
	http.HandleFunc("/login", web.LoginHandler)
	http.HandleFunc("/register", web.RegisterPage)
	http.HandleFunc("/register-handler", web.RegisterHandler)
	http.HandleFunc("/chat", web.AuthMiddleware(web.ChatPage))
	http.HandleFunc("/logout", web.LogoutHandler) // Новый маршрут для выхода
	http.HandleFunc("/ws", web.AuthMiddleware(web.WebSocketHandler))

	// Статические файлы
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Println("📊 База данных: PostgreSQL")
	log.Println("Тестовые учетные записи: test/test, admin/admin")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
