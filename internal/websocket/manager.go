package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"pigeongram/pkg/models"
	"pigeongram/repository/cache"
	"pigeongram/repository/postgres"

	"github.com/go-redis/redis/v8"
)

// Типы сообщений WebSocket
const (
	TypeMessage   = "message" // новое сообщение в чате
	TypeFileEvent = "file"    // событие с файлами (загрузка/удаление)
)

// FileEventData - данные о событии с файлами
type FileEventData struct {
	EventType string      `json:"eventType"` // "upload" или "delete"
	ChatID    string      `json:"chatId"`
	File      interface{} `json:"file"` // информация о файле
	Username  string      `json:"username"`
	Timestamp time.Time   `json:"timestamp"`
}

// Manager управляет WebSocket соединениями
type Manager struct {
	Clients   map[*Client]bool
	ClientsMu sync.RWMutex

	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan models.Message
	FileEvents chan FileEventData // для файловых событий

	// Репозитории
	MsgRepo  *postgres.MessageRepository
	UserRepo *postgres.UserRepository
	Cache    cache.Cache

	// Redis Pub/Sub
	RedisClient *redis.Client
	PubSub      *redis.PubSub
	ServerID    string

	// Канал для сообщений из Redis
	RedisMsgCh chan []byte
}

// NewManager создает новый менеджер
func NewManager(
	msgRepo *postgres.MessageRepository,
	userRepo *postgres.UserRepository,
	cache cache.Cache,
	redisClient *redis.Client,
	serverID string,
) *Manager {
	return &Manager{
		Clients:     make(map[*Client]bool),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Broadcast:   make(chan models.Message, 100),
		FileEvents:  make(chan FileEventData, 100),
		MsgRepo:     msgRepo,
		UserRepo:    userRepo,
		Cache:       cache,
		RedisClient: redisClient,
		ServerID:    serverID,
		RedisMsgCh:  make(chan []byte, 100),
	}
}

// Run запускает менеджер
func (m *Manager) Run(ctx context.Context) {
	log.Printf("🚀 WebSocket менеджер запущен (ServerID: %s)", m.ServerID)

	// Подписываемся на Redis Pub/Sub
	if m.RedisClient != nil {
		m.subscribeToRedis(ctx)
	}

	for {
		select {
		case client := <-m.Register:
			m.ClientsMu.Lock()
			m.Clients[client] = true
			m.ClientsMu.Unlock()

			log.Printf("🔌 Клиент %s подключился (всего: %d)",
				client.Username, len(m.Clients))

			// Отправляем историю сообщений
			go m.sendMessageHistory(client)

		case client := <-m.Unregister:
			m.ClientsMu.Lock()
			if _, ok := m.Clients[client]; ok {
				delete(m.Clients, client)
				close(client.Send)
				log.Printf("🔌 Клиент %s отключился (всего: %d)",
					client.Username, len(m.Clients))
			}
			m.ClientsMu.Unlock()

		case message := <-m.Broadcast:
			// Сохраняем в БД
			go m.saveMessageToDB(message)

			// Обновляем кэш
			go m.updateCache(message)

			// Отправляем через Redis другим серверам
			m.publishToRedis(message)

			// Отправляем локальным клиентам
			m.broadcastToLocalClients(message)

		case fileEvent := <-m.FileEvents: // 👈 НОВЫЙ ОБРАБОТЧИК
			// Сохраняем в кэш? (опционально)

			// Публикуем в Redis для других серверов
			m.publishFileEventToRedis(fileEvent)

			// Отправляем локальным клиентам
			m.broadcastFileEventToLocalClients(fileEvent)

		case redisMsg := <-m.RedisMsgCh:
			// Получили сообщение от другого сервера
			var wsMsg struct {
				ServerID  string          `json:"server_id"`
				Timestamp time.Time       `json:"timestamp"`
				Data      json.RawMessage `json:"data"`
			}

			if err := json.Unmarshal(redisMsg, &wsMsg); err != nil {
				log.Printf("❌ Ошибка парсинга Redis сообщения: %v", err)
				continue
			}

			// Игнорируем свои сообщения
			if wsMsg.ServerID == m.ServerID {
				continue
			}

			// Парсим сообщение
			var message models.Message
			if err := json.Unmarshal(wsMsg.Data, &message); err != nil {
				log.Printf("❌ Ошибка парсинга сообщения: %v", err)
				continue
			}

			// Отправляем локальным клиентам
			m.broadcastToLocalClients(message)
		}
	}
}

// publishFileEventToRedis публикует событие о файле в Redis
func (m *Manager) publishFileEventToRedis(event FileEventData) {
	if m.RedisClient == nil {
		log.Printf("⚠️ Redis не доступен, событие не будет отправлено другим серверам")
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("❌ Ошибка сериализации файлового события: %v", err)
		return
	}

	wsMsg := struct {
		Type     string          `json:"type"`
		ServerID string          `json:"server_id"`
		Data     json.RawMessage `json:"data"`
	}{
		Type:     "file",
		ServerID: m.ServerID,
		Data:     data,
	}

	jsonData, _ := json.Marshal(wsMsg)

	// Публикуем в оба канала для надежности
	err = m.RedisClient.Publish(context.Background(), "chat:files", jsonData).Err()
	if err != nil {
		log.Printf("❌ Ошибка публикации в Redis канал chat:files: %v", err)
	} else {
		log.Printf("📤 Событие опубликовано в Redis канал chat:files")
	}

	// Также публикуем в общий канал для обратной совместимости
	err = m.RedisClient.Publish(context.Background(), "chat:messages", jsonData).Err()
	if err != nil {
		log.Printf("❌ Ошибка публикации в Redis канал chat:messages: %v", err)
	}
}

// broadcastFileEventToLocalClients рассылает событие локальным клиентам
func (m *Manager) broadcastFileEventToLocalClients(event FileEventData) {
	m.ClientsMu.RLock()
	defer m.ClientsMu.RUnlock()

	message := map[string]interface{}{
		"type": "file",
		"data": event,
	}

	clientsCount := 0
	for client := range m.Clients {
		// Можно фильтровать по чату, если нужно
		select {
		case client.SendFileEvent <- message:
			clientsCount++
		default:
			log.Printf("⚠️ Канал клиента %s переполнен, пропускаем", client.Username)
		}
	}

	log.Printf("📢 Файловое событие разослано %d клиентам", clientsCount)
}

// subscribeToRedis подписывается на каналы Redis
func (m *Manager) subscribeToRedis(ctx context.Context) {
	m.PubSub = m.RedisClient.Subscribe(ctx, "chat:messages", "chat:files") // 👈 добавили "chat:files"

	go func() {
		for {
			msg, err := m.PubSub.ReceiveMessage(ctx)
			if err != nil {
				log.Printf("❌ Ошибка Redis Pub/Sub: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var wsMsg struct {
				Type     string          `json:"type"`
				ServerID string          `json:"server_id"`
				Data     json.RawMessage `json:"data"`
			}

			if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
				log.Printf("❌ Ошибка парсинга Redis сообщения: %v", err)
				continue
			}

			// Игнорируем свои сообщения
			if wsMsg.ServerID == m.ServerID {
				continue
			}

			switch wsMsg.Type {
			case TypeFileEvent:
				var event FileEventData
				if err := json.Unmarshal(wsMsg.Data, &event); err != nil {
					log.Printf("❌ Ошибка парсинга файлового события: %v", err)
					continue
				}
				m.broadcastFileEventToLocalClients(event)

			case TypeMessage:
				// обработка текстовых сообщений (уже есть)
			}
		}
	}()
}

// publishToRedis публикует сообщение в Redis
func (m *Manager) publishToRedis(message models.Message) {
	if m.RedisClient == nil {
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Ошибка сериализации для Redis: %v", err)
		return
	}

	wsMsg := struct {
		ServerID  string    `json:"server_id"`
		Timestamp time.Time `json:"timestamp"`
		Data      []byte    `json:"data"`
	}{
		ServerID:  m.ServerID,
		Timestamp: time.Now(),
		Data:      data,
	}

	jsonData, _ := json.Marshal(wsMsg)

	err = m.RedisClient.Publish(context.Background(), "chat:messages", jsonData).Err()
	if err != nil {
		log.Printf("❌ Ошибка публикации в Redis: %v", err)
	}
}

// broadcastToLocalClients рассылает сообщение локальным клиентам
func (m *Manager) broadcastToLocalClients(message models.Message) {
	m.ClientsMu.RLock()
	defer m.ClientsMu.RUnlock()

	for client := range m.Clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(m.Clients, client)
		}
	}
}

// sendMessageHistory отправляет историю сообщений клиенту
func (m *Manager) sendMessageHistory(client *Client) {
	ctx := context.Background()
	var messages []models.Message

	// Сначала пробуем из кэша
	if m.Cache != nil {
		cached, err := m.Cache.GetRecentMessages(ctx)
		if err == nil && cached != nil {
			messages = cached
			log.Printf("📦 Загружено %d сообщений из кэша для %s",
				len(messages), client.Username)
		}
	}

	// Если в кэше нет, грузим из БД
	if len(messages) == 0 && m.MsgRepo != nil {
		recent, err := m.MsgRepo.GetRecent(ctx, 100)
		if err == nil {
			for _, msg := range recent {
				messages = append(messages, models.Message{
					Username:  msg.User.Username,
					Text:      msg.Content,
					Timestamp: msg.Timestamp,
				})
			}
			log.Printf("📜 Загружено %d сообщений из БД для %s",
				len(messages), client.Username)

			// Сохраняем в кэш
			if m.Cache != nil && len(messages) > 0 {
				m.Cache.SetRecentMessages(ctx, messages)
			}
		}
	}

	// Сортируем по времени
	for i := 0; i < len(messages)-1; i++ {
		for j := i + 1; j < len(messages); j++ {
			if messages[i].Timestamp.After(messages[j].Timestamp) {
				messages[i], messages[j] = messages[j], messages[i]
			}
		}
	}

	// Отправляем клиенту
	for _, msg := range messages {
		client.Send <- msg
	}
}

// saveMessageToDB сохраняет сообщение в БД
func (m *Manager) saveMessageToDB(message models.Message) {
	if m.MsgRepo == nil {
		return
	}

	ctx := context.Background()
	err := m.MsgRepo.Create(ctx, &message)
	if err != nil {
		log.Printf("❌ Ошибка сохранения в БД: %v", err)
	}
}

// updateCache обновляет кэш
func (m *Manager) updateCache(message models.Message) {
	if m.Cache == nil {
		return
	}

	ctx := context.Background()

	// Получаем текущий кэш
	cached, err := m.Cache.GetRecentMessages(ctx)
	if err == nil && cached != nil {
		// Добавляем новое сообщение
		updated := append(cached, message)

		// Оставляем последние 100
		if len(updated) > 100 {
			updated = updated[len(updated)-100:]
		}

		m.Cache.SetRecentMessages(ctx, updated)
	} else {
		m.Cache.SetRecentMessages(ctx, []models.Message{message})
	}
}

// GetStats возвращает статистику менеджера
func (m *Manager) GetStats() map[string]interface{} {
	m.ClientsMu.RLock()
	defer m.ClientsMu.RUnlock()

	return map[string]interface{}{
		"server_id":     m.ServerID,
		"clients_count": len(m.Clients),
		"clients":       getClientUsernames(m.Clients),
	}
}

func getClientUsernames(clients map[*Client]bool) []string {
	usernames := make([]string, 0, len(clients))
	for client := range clients {
		usernames = append(usernames, client.Username)
	}
	return usernames
}
