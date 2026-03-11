package main

import (
	"log"
	"net/http"
)

func main() {
	// Инициализация хранилищ
	InitUserStore()
	InitMessageStore()

	// Настройка маршрутов
	http.HandleFunc("/", LoginPage)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/register", RegisterPage)
	http.HandleFunc("/register-handler", RegisterHandler)
	http.HandleFunc("/chat", ChatPage)
	http.HandleFunc("/send-message", SendMessageHandler)
	http.HandleFunc("/get-messages", GetMessagesHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
