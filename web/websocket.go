package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"pigeongram/pkg/models" // DTO для клиента
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn     *websocket.Conn
	send     chan models.Message // отправляем DTO
	username string
}

var (
	clients    = make(map[*Client]bool)
	broadcast  = make(chan models.Message) // канал для DTO
	register   = make(chan *Client)
	unregister = make(chan *Client)
)

func InitWebSocket() {
	go run()
	log.Println("🔌 WebSocket менеджер запущен")
}

func run() {
	for {
		select {
		case client := <-register:
			clients[client] = true
			log.Printf("🔌 Клиент %s подключился (всего: %d)", client.username, len(clients))

			// Отправляем историю
			if err := sendMessageHistory(client); err != nil {
				log.Printf("❌ Ошибка отправки истории: %v", err)
			}

		case client := <-unregister:
			if _, ok := clients[client]; ok {
				delete(clients, client)
				close(client.send)
				log.Printf("🔌 Клиент %s отключился (всего: %d)", client.username, len(clients))
			}

		case message := <-broadcast:
			// Сохраняем в БД
			go saveMessageToDB(message)

			// Обновляем кэш - добавляем новое сообщение
			if messageCache != nil {
				ctx := context.Background()

				// Получаем текущий кэш
				cached, err := messageCache.GetRecentMessages(ctx)
				if err == nil && cached != nil {
					// Добавляем новое сообщение в конец (оно самое новое)
					updatedMessages := append(cached, message)

					// Оставляем только последние 100 сообщений
					if len(updatedMessages) > 100 {
						updatedMessages = updatedMessages[len(updatedMessages)-100:]
					}

					// Сохраняем обновленный кэш
					messageCache.SetRecentMessages(ctx, updatedMessages)
					log.Printf("📦 Кэш обновлен: добавлено сообщение от %s", message.Username)
				} else {
					// Если кэша нет, создаем новый с этим сообщением
					messageCache.SetRecentMessages(ctx, []models.Message{message})
				}
			}

			// Рассылаем всем
			for client := range clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(clients, client)
				}
			}
		}
	}
}

// sendMessageHistory отправляет историю сообщений клиенту в правильном порядке
func sendMessageHistory(client *Client) error {
	ctx := context.Background()
	var messages []models.Message // DTO для клиента

	// Сначала пробуем получить из кэша
	var cachedMessages []models.Message
	if messageCache != nil {
		cached, err := messageCache.GetRecentMessages(ctx)
		if err == nil && cached != nil {
			cachedMessages = cached
			log.Printf("📦 Загружено %d сообщений из КЭША для %s", len(cachedMessages), client.username)

			// Убеждаемся, что сообщения отсортированы правильно (старые сначала)
			for i := 0; i < len(cachedMessages)-1; i++ {
				for j := i + 1; j < len(cachedMessages); j++ {
					if cachedMessages[i].Timestamp.After(cachedMessages[j].Timestamp) {
						cachedMessages[i], cachedMessages[j] = cachedMessages[j], cachedMessages[i]
					}
				}
			}
			messages = cachedMessages
		}
	}

	// Если в кэше нет или там меньше сообщений, грузим из БД
	if (len(messages) == 0 || len(messages) < 100) && msgRepo != nil {
		recent, err := msgRepo.GetRecent(ctx, 100)
		if err == nil {
			// Конвертируем Entity в DTO
			dbMessages := make([]models.Message, 0, len(recent))
			for _, msg := range recent {
				dbMessages = append(dbMessages, models.Message{
					Username:  msg.User.Username,
					Text:      msg.Content,
					Timestamp: msg.Timestamp,
				})
			}

			// Сортируем сообщения по возрастанию времени (старые сначала)
			for i := 0; i < len(dbMessages)-1; i++ {
				for j := i + 1; j < len(dbMessages); j++ {
					if dbMessages[i].Timestamp.After(dbMessages[j].Timestamp) {
						dbMessages[i], dbMessages[j] = dbMessages[j], dbMessages[i]
					}
				}
			}

			// Если были сообщения из кэша, объединяем
			if len(messages) > 0 {
				// Создаем мапу для уникальности по тексту+времени (простой способ)
				seen := make(map[string]bool)
				for _, msg := range messages {
					key := msg.Username + msg.Text + msg.Timestamp.String()
					seen[key] = true
				}

				// Добавляем новые сообщения из БД
				for _, msg := range dbMessages {
					key := msg.Username + msg.Text + msg.Timestamp.String()
					if !seen[key] {
						messages = append(messages, msg)
					}
				}

				// Пересортировываем
				for i := 0; i < len(messages)-1; i++ {
					for j := i + 1; j < len(messages); j++ {
						if messages[i].Timestamp.After(messages[j].Timestamp) {
							messages[i], messages[j] = messages[j], messages[i]
						}
					}
				}
			} else {
				messages = dbMessages
			}

			log.Printf("📜 Загружено %d сообщений из БД для %s", len(dbMessages), client.username)

			// Обновляем кэш (только если мы загрузили больше, чем было)
			if messageCache != nil && len(messages) > len(cachedMessages) {
				messageCache.SetRecentMessages(ctx, messages)
				log.Printf("📦 Кэш обновлен из БД: теперь %d сообщений", len(messages))
			}
		}
	}

	// Отправляем клиенту
	for _, msg := range messages {
		client.send <- msg
	}

	return nil
}

// saveMessageToDB сохраняет сообщение в БД
func saveMessageToDB(msg models.Message) {
	if msgRepo == nil {
		return
	}

	ctx := context.Background()

	// Сохраняем через репозиторий
	err := msgRepo.Create(ctx, &msg)
	if err != nil {
		log.Printf("❌ Ошибка сохранения сообщения в БД: %v", err)
	} else {
		log.Printf("💾 Сообщение от %s сохранено в БД", msg.Username)
	}
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("❌ Ошибка upgrade:", err)
		return
	}

	client := &Client{
		conn:     conn,
		send:     make(chan models.Message, 256),
		username: username,
	}

	register <- client

	go writePump(client)
	go readPump(client)
}

func readPump(client *Client) {
	defer func() {
		unregister <- client
		client.conn.Close()
	}()

	for {
		var msg struct {
			Text string `json:"text"`
		}
		err := client.conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		if msg.Text != "" {
			// Создаем DTO для отправки в канал
			broadcast <- models.Message{
				Username:  client.username,
				Text:      msg.Text,
				Timestamp: time.Now(),
			}
		}
	}
}

func writePump(client *Client) {
	defer client.conn.Close()

	for message := range client.send {
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("❌ Ошибка сериализации: %v", err)
			continue
		}

		err = client.conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			break
		}
	}
}
