package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"pigeongram/models"

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
	Send     chan interface{}
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

// Добавляем функцию для сохранения сообщений в БД
func saveMessageToDB(msg Message) {
	if msgRepo == nil {
		return
	}

	ctx := context.Background()

	// Находим пользователя
	user, err := userRepo.GetByUsername(ctx, msg.Username)
	if err != nil || user == nil {
		log.Printf("❌ Не удалось найти пользователя %s в БД", msg.Username)
		return
	}

	// Создаем сообщение для БД
	dbMsg := &models.Message{
		UserID:    user.ID,
		Content:   msg.Text,
		Timestamp: msg.Timestamp,
	}

	// Сохраняем
	err = msgRepo.Create(ctx, dbMsg)
	if err != nil {
		log.Printf("❌ Ошибка сохранения сообщения в БД: %v", err)
	} else {
		log.Printf("💾 Сообщение сохранено в БД")
	}
}

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
			m.Clients[client] = true
			log.Printf("🔌 Клиент %s подключился (всего: %d)", client.Username, len(m.Clients))

			// Загружаем историю из БД
			if msgRepo != nil {
				ctx := context.Background()
				recent, err := msgRepo.GetRecent(ctx, 100)
				if err == nil {
					for _, msg := range recent {
						// Конвертируем из модели в старый формат
						oldMsg := Message{
							Username:  msg.User.Username,
							Text:      msg.Content,
							Timestamp: msg.Timestamp,
						}
						client.Send <- oldMsg
					}
					log.Printf("📜 Загружено %d сообщений из БД", len(recent))
				}
			} else {
				// Fallback на старую систему
				messagesMu.RLock()
				for _, msg := range messages {
					data, _ := json.Marshal(msg)
					client.Send <- data
				}
				messagesMu.RUnlock()
			}

		case client := <-m.Unregister:
			if _, ok := m.Clients[client]; ok {
				delete(m.Clients, client)
				close(client.Send)
				log.Printf("🔌 Клиент %s отключился (всего: %d)", client.Username, len(m.Clients))
			}

		case message := <-m.Broadcast:
			// Сохраняем в старую систему
			messagesMu.Lock()
			messages = append(messages, message)
			messagesMu.Unlock()

			// Сохраняем в БД
			go saveMessageToDB(message)

			data, _ := json.Marshal(message)
			for client := range m.Clients {
				select {
				case client.Send <- data:
				default:
					close(client.Send)
					delete(m.Clients, client)
				}
			}
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
		Send:     make(chan interface{}, 256),
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

	for message := range c.Send {
		// message может быть Message, []byte, или чем угодно
		var data []byte
		var err error

		switch v := message.(type) {
		case []byte:
			data = v // уже сериализовано
		default:
			data, err = json.Marshal(v) // сериализуем
		}

		if err != nil {
			log.Printf("❌ Ошибка сериализации: %v", err)
			continue
		}

		c.Conn.WriteMessage(websocket.TextMessage, data)
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
