package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	// Инициализация генератора случайных чисел
	rand.Seed(time.Now().UnixNano())

	// Инициализация хранилищ
	InitUserStore()
	InitMessageStore()
	InitWebSocket()

	// Настройка маршрутов
	http.HandleFunc("/", LoginPage)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/register", RegisterPage)
	http.HandleFunc("/register-handler", RegisterHandler)
	http.HandleFunc("/chat", AuthMiddleware(ChatPage))
	http.HandleFunc("/logout", LogoutHandler) // Новый маршрут для выхода
	http.HandleFunc("/ws", AuthMiddleware(WebSocketHandler))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Println("Тестовые учетные записи: test/test, admin/admin")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
