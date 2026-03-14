package web

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	ws "pigeongram/internal/websocket" // алиас для нашего пакета
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var wsManager *ws.Manager

// InitWebSocket инициализирует WebSocket менеджер
func InitWebSocket(manager *ws.Manager) {
	wsManager = manager
	log.Println("🔌 WebSocket адаптер инициализирован")
}

// WebSocketHandler обрабатывает WebSocket соединения
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	username := getUserFromSession(r)
	if username == "" {
		log.Printf("❌ Неавторизованная попытка подключения к WebSocket")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP до WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ Ошибка upgrade до WebSocket: %v", err)
		return
	}

	// Создаем клиента через наш конструктор
	client := ws.NewClient(conn, username, wsManager)

	// Регистрируем клиента
	wsManager.Register <- client

	// Запускаем горутины для чтения и записи
	go client.ReadPump()
	go client.WritePump()

	log.Printf("🔌 Клиент %s подключился к WebSocket", username)
}
