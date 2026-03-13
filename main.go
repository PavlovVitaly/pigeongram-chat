package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"pigeongram/web"
	"time"

	"pigeongram/config"
	"pigeongram/repository/postgres"
)

func main() {
	// Инициализация генератора случайных чисел
	rand.Seed(time.Now().UnixNano())

	// 1. Инициализация PostgreSQL
	log.Println("📦 Подключение к PostgreSQL...")
	dbConfig := config.NewDefaultConfig()
	db, err := config.InitPostgres(dbConfig)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	log.Println("✅ PostgreSQL подключен успешно")

	// 2. Создаем репозитории
	userRepo := postgres.NewUserRepository(db)
	msgRepo := postgres.NewMessageRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)

	// 3. Инициализируем хранилища в web пакете
	// TODO: Пока оставляем и старую память, и новую БД для плавного перехода
	web.InitStores(userRepo, msgRepo, sessionRepo)

	// 4. Запускаем периодическую очистку сессий
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

	// 5. Запускаем WebSocket менеджер
	web.InitWebSocket()

	// Настройка маршрутов
	http.HandleFunc("/", web.LoginPage)
	http.HandleFunc("/login", web.LoginHandler)
	http.HandleFunc("/register", web.RegisterPage)
	http.HandleFunc("/register-handler", web.RegisterHandler)
	http.HandleFunc("/chat", web.AuthMiddleware(web.ChatPage))
	http.HandleFunc("/logout", web.LogoutHandler) // Новый маршрут для выхода
	http.HandleFunc("/ws", web.AuthMiddleware(web.WebSocketHandler))
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Println("📊 База данных: PostgreSQL")
	log.Println("Тестовые учетные записи: test/test, admin/admin")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
