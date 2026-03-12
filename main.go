package main

import (
	"log"
	"math/rand"
	"net/http"
	"pigeongram/web"
	"time"
)

func main() {
	// Инициализация генератора случайных чисел
	rand.Seed(time.Now().UnixNano())

	// Инициализация хранилищ
	web.InitUserStore()
	web.InitMessageStore()
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
	log.Println("Тестовые учетные записи: test/test, admin/admin")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
