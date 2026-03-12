package web

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем все источники для разработки
	},
}

// Client представляет подключенного клиента
type Client struct {
	Conn     *websocket.Conn
	Username string
	Send     chan Message
}

// WebSocket менеджер
type WebSocketManager struct {
	Clients    map[*Client]bool
	Broadcast  chan Message
	Register   chan *Client
	Unregister chan *Client
	Mutex      sync.RWMutex
}

var manager *WebSocketManager

// Инициализация WebSocket менеджера
func InitWebSocket() {
	manager = &WebSocketManager{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}

	go manager.Run()
}

// Запуск менеджера
func (m *WebSocketManager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.Mutex.Lock()
			m.Clients[client] = true
			m.Mutex.Unlock()

			// Отправляем историю сообщений новому клиенту
			messagesMu.RLock()
			for _, msg := range messages {
				client.Send <- msg
			}
			messagesMu.RUnlock()

			log.Printf("Клиент %s подключился. Всего клиентов: %d", client.Username, len(m.Clients))

		case client := <-m.Unregister:
			m.Mutex.Lock()
			if _, ok := m.Clients[client]; ok {
				delete(m.Clients, client)
				close(client.Send)
				log.Printf("Клиент %s отключился. Всего клиентов: %d", client.Username, len(m.Clients))
			}
			m.Mutex.Unlock()

		case message := <-m.Broadcast:
			// Сохраняем сообщение в историю
			messagesMu.Lock()
			messages = append(messages, message)
			messagesMu.Unlock()

			// Рассылаем всем клиентам
			m.Mutex.RLock()
			for client := range m.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(m.Clients, client)
				}
			}
			m.Mutex.RUnlock()
		}
	}
}

// WebSocket обработчик
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка аутентификации
	username := getSessionUser(r)
	if username == "" {
		log.Printf("Неавторизованная попытка подключения к WebSocket")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Устанавливаем WebSocket соединение
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Ошибка установки WebSocket соединения: %v", err)
		return
	}

	// Создаем клиента
	client := &Client{
		Conn:     conn,
		Username: username,
		Send:     make(chan Message, 256),
	}

	// Регистрируем клиента
	manager.Register <- client

	// Запускаем горутины для чтения и записи
	go client.WritePump()
	go client.ReadPump()

	log.Printf("Пользователь %s подключился к WebSocket", username)
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Отправляем сообщение в JSON формате
			err := c.Conn.WriteJSON(message)
			if err != nil {
				log.Printf("Ошибка отправки сообщения клиенту %s: %v", c.Username, err)
				return
			}
		}
	}
}

// ReadPump читает сообщения от клиента
func (c *Client) ReadPump() {
	defer func() {
		manager.Unregister <- c
		c.Conn.Close()
	}()

	// Устанавливаем лимиты
	c.Conn.SetReadLimit(512) // Максимальный размер сообщения 512 байт
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg struct {
			Text string `json:"text"`
		}

		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Ошибка чтения сообщения от %s: %v", c.Username, err)
			}
			break
		}

		// Проверяем, что сообщение не пустое
		if msg.Text == "" {
			continue
		}

		// Создаем сообщение для рассылки
		message := Message{
			Username:  c.Username,
			Text:      msg.Text,
			Timestamp: time.Now(),
		}

		// Отправляем в канал broadcast
		manager.Broadcast <- message
	}
}
