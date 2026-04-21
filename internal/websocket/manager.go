package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"pigeongram-chat/internal/metrics"
	"pigeongram-chat/internal/storage"
	"pigeongram-chat/pkg/models"
	"pigeongram-chat/repository/cache"
	"pigeongram-chat/repository/postgres"

	"github.com/go-redis/redis/v8"
)

// Типы сообщений WebSocket
const (
	TypeMessage   = "message"
	TypeFileEvent = "file"
	TypeOnline    = "online"
	TypeTyping    = "typing"
	TypeSystem    = "system"
)

// OnlineData - данные о онлайн пользователях
type OnlineData struct {
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// FileEventData - данные о событиях с файлами
type FileEventData struct {
	EventType string      `json:"eventType"` // "upload" или "delete"
	ChatID    string      `json:"chatId"`
	File      interface{} `json:"file"`
	Username  string      `json:"username"`
	Owner     string      `json:"owner"` // владелец файла
	Timestamp time.Time   `json:"timestamp"`
}

// Manager управляет WebSocket соединениями
type Manager struct {
	Clients   map[*Client]bool
	ClientsMu sync.RWMutex

	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan models.Message
	FileEvents chan FileEventData

	OnlineUsers map[string]*Client
	OnlineMu    sync.RWMutex

	MsgRepo  *postgres.MessageRepository
	UserRepo *postgres.UserRepository
	Cache    cache.Cache

	RedisClient *redis.Client
	PubSub      *redis.PubSub
	ServerID    string
	RedisMsgCh  chan []byte
}

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
		OnlineUsers: make(map[string]*Client),
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

	metrics.OnlineUsers.Set(0)
	metrics.ActiveConnections.Set(0)

	if m.RedisClient != nil {
		m.subscribeToRedis(ctx)
	}

	go m.broadcastOnlineUsersPeriodically(ctx)
	go m.collectDBStatsPeriodically(ctx)

	for {
		select {
		case client := <-m.Register:
			metrics.ActiveConnections.Inc()

			m.ClientsMu.Lock()
			m.Clients[client] = true
			m.ClientsMu.Unlock()

			m.OnlineMu.Lock()
			m.OnlineUsers[client.Username] = client
			onlineCount := len(m.OnlineUsers)
			m.OnlineMu.Unlock()

			metrics.OnlineUsers.Set(float64(onlineCount))

			log.Printf("🔌 Клиент %s подключился (всего: %d, онлайн: %d)",
				client.Username, len(m.Clients), onlineCount)

			m.broadcastOnlineUsers()
			go m.sendMessageHistory(client)

		case client := <-m.Unregister:
			m.ClientsMu.Lock()
			if _, ok := m.Clients[client]; ok {
				delete(m.Clients, client)
				close(client.Send)
				close(client.SendFileEvent)
				close(client.SendOnline)

				metrics.ActiveConnections.Dec()
			}
			m.ClientsMu.Unlock()

			m.OnlineMu.Lock()
			delete(m.OnlineUsers, client.Username)
			onlineCount := len(m.OnlineUsers)
			m.OnlineMu.Unlock()

			metrics.OnlineUsers.Set(float64(onlineCount))

			log.Printf("🔌 Клиент %s отключился (всего: %d, онлайн: %d)",
				client.Username, len(m.Clients), onlineCount)

			m.broadcastOnlineUsers()

		case message := <-m.Broadcast:
			// Фильтрация пустых сообщений
			if message.Text == "" {
				log.Printf("⚠️ [MANAGER] Попытка отправить пустое сообщение, игнорируется")
				continue
			}

			if message.Username == "" {
				message.Username = "system"
			}

			if message.Timestamp.IsZero() {
				message.Timestamp = time.Now()
			}

			metrics.MessagesTotal.Inc()

			startDB := time.Now()
			go m.saveMessageToDB(message)
			metrics.DatabaseDuration.WithLabelValues("create_message").Observe(time.Since(startDB).Seconds())

			go m.updateCache(message)

			if m.RedisClient != nil {
				m.publishToRedis(message)
			}

			m.broadcastToLocalClients(message)

		case fileEvent := <-m.FileEvents:
			log.Printf("📁 Файловое событие: %s в чате %s от %s (владелец: %s)",
				fileEvent.EventType, fileEvent.ChatID, fileEvent.Username, fileEvent.Owner)

			if fileEvent.EventType == "upload" {
				if fileInfo, ok := fileEvent.File.(storage.FileInfo); ok {
					metrics.FilesUploadedTotal.Inc()
					metrics.FilesSizeBytes.Add(float64(fileInfo.Size))
					log.Printf("📊 Метрики файла: размер %d байт", fileInfo.Size)
				}
			}

			if m.RedisClient != nil {
				m.publishFileEventToRedis(fileEvent)
			}
			m.broadcastFileEventToLocalClients(fileEvent)
		}
	}
}

// sendMessageHistory отправляет историю сообщений клиенту
func (m *Manager) sendMessageHistory(client *Client) {
	ctx := context.Background()
	var messages []models.Message

	start := time.Now()
	defer func() {
		metrics.DatabaseDuration.WithLabelValues("get_history").Observe(time.Since(start).Seconds())
	}()

	// Пробуем из кэша
	if m.Cache != nil {
		cached, err := m.Cache.GetRecentMessages(ctx)
		if err == nil && cached != nil {
			messages = cached
			metrics.CacheOperations.WithLabelValues("get", "hit").Inc()
			log.Printf("📦 Загружено %d сообщений из кэша для %s", len(messages), client.Username)
		} else {
			metrics.CacheOperations.WithLabelValues("get", "miss").Inc()
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
			log.Printf("📜 Загружено %d сообщений из БД для %s", len(messages), client.Username)

			if m.Cache != nil && len(messages) > 0 {
				m.Cache.SetRecentMessages(ctx, messages)
				metrics.CacheOperations.WithLabelValues("set", "success").Inc()
			}
		}
	}

	// Фильтрация пустых сообщений
	validMessages := make([]models.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Text != "" && msg.Text != "null" && msg.Text != "undefined" {
			if msg.Username == "" {
				msg.Username = "system"
			}
			if msg.Timestamp.IsZero() {
				msg.Timestamp = time.Now()
			}
			validMessages = append(validMessages, msg)
		}
	}

	// Сортируем по времени
	for i := 0; i < len(validMessages)-1; i++ {
		for j := i + 1; j < len(validMessages); j++ {
			if validMessages[i].Timestamp.After(validMessages[j].Timestamp) {
				validMessages[i], validMessages[j] = validMessages[j], validMessages[i]
			}
		}
	}

	log.Printf("📜 [HISTORY] Отправка %d сообщений %s (отфильтровано из %d)",
		len(validMessages), client.Username, len(messages))

	for _, msg := range validMessages {
		select {
		case client.Send <- msg:
		default:
			log.Printf("⚠️ Канал клиента %s переполнен при отправке истории", client.Username)
		}
	}
}

// saveMessageToDB сохраняет сообщение в БД
func (m *Manager) saveMessageToDB(message models.Message) {
	if m.MsgRepo == nil {
		return
	}

	ctx := context.Background()

	start := time.Now()
	defer func() {
		metrics.DatabaseDuration.WithLabelValues("save_message").Observe(time.Since(start).Seconds())
	}()

	err := m.MsgRepo.Create(ctx, &message)
	if err != nil {
		log.Printf("❌ Ошибка сохранения в БД: %v", err)
		metrics.DatabaseOperations.WithLabelValues("create", "error").Inc()
	} else {
		metrics.DatabaseOperations.WithLabelValues("create", "success").Inc()
	}
}

// updateCache обновляет кэш
func (m *Manager) updateCache(message models.Message) {
	if m.Cache == nil {
		return
	}

	ctx := context.Background()

	start := time.Now()
	defer func() {
		metrics.DatabaseDuration.WithLabelValues("update_cache").Observe(time.Since(start).Seconds())
	}()

	cached, err := m.Cache.GetRecentMessages(ctx)
	if err == nil && cached != nil {
		updated := append(cached, message)
		if len(updated) > 100 {
			updated = updated[len(updated)-100:]
		}
		m.Cache.SetRecentMessages(ctx, updated)
		metrics.CacheOperations.WithLabelValues("update", "success").Inc()
	} else {
		m.Cache.SetRecentMessages(ctx, []models.Message{message})
		metrics.CacheOperations.WithLabelValues("set", "success").Inc()
	}
}

// broadcastOnlineUsersPeriodically периодически обновляет список онлайн
func (m *Manager) broadcastOnlineUsersPeriodically(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.broadcastOnlineUsers()

			m.OnlineMu.RLock()
			onlineCount := len(m.OnlineUsers)
			m.OnlineMu.RUnlock()

			metrics.OnlineUsers.Set(float64(onlineCount))
			metrics.ActiveConnections.Set(float64(len(m.Clients)))
		}
	}
}

// collectDBStatsPeriodically собирает статистику БД
func (m *Manager) collectDBStatsPeriodically(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("📊 Остановка сбора статистики БД")
			return
		case <-ticker.C:
			if m.MsgRepo == nil || m.UserRepo == nil {
				log.Println("⚠️ Репозитории не инициализированы, пропускаем сбор статистики")
				continue
			}

			queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

			if count, err := m.UserRepo.Count(queryCtx); err == nil {
				metrics.TotalUsers.Set(float64(count))
				log.Printf("📊 Всего пользователей: %d", count)
			} else {
				log.Printf("❌ Ошибка получения количества пользователей: %v", err)
			}

			if count, err := m.MsgRepo.Count(queryCtx); err == nil {
				metrics.TotalMessages.Set(float64(count))
				log.Printf("📊 Всего сообщений: %d", count)
			} else {
				log.Printf("❌ Ошибка получения количества сообщений: %v", err)
			}

			cancel()
		}
	}
}

// broadcastOnlineUsers рассылает список онлайн пользователей
func (m *Manager) broadcastOnlineUsers() {
	m.OnlineMu.RLock()
	users := make([]string, 0, len(m.OnlineUsers))
	for username := range m.OnlineUsers {
		users = append(users, username)
	}
	m.OnlineMu.RUnlock()

	data := OnlineData{
		Count: len(users),
		Users: users,
	}

	message := map[string]interface{}{
		"type": "online",
		"data": data,
	}

	m.ClientsMu.RLock()
	defer m.ClientsMu.RUnlock()

	for client := range m.Clients {
		select {
		case client.SendOnline <- message:
		default:
			log.Printf("⚠️ Не удалось отправить онлайн статус клиенту %s (канал переполнен)",
				client.Username)
		}
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
			metrics.WebSocketErrors.WithLabelValues("send_queue_full").Inc()
			log.Printf("⚠️ Канал клиента %s переполнен", client.Username)
		}
	}
}

// broadcastFileEventToLocalClients рассылает файловые события
func (m *Manager) broadcastFileEventToLocalClients(event FileEventData) {
	m.ClientsMu.RLock()
	defer m.ClientsMu.RUnlock()

	message := map[string]interface{}{
		"type": "file",
		"data": event,
	}

	for client := range m.Clients {
		select {
		case client.SendFileEvent <- message:
		default:
			metrics.WebSocketErrors.WithLabelValues("file_queue_full").Inc()
		}
	}
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
		ServerID string `json:"server_id"`
		Data     []byte `json:"data"`
	}{
		ServerID: m.ServerID,
		Data:     data,
	}

	jsonData, _ := json.Marshal(wsMsg)

	err = m.RedisClient.Publish(context.Background(), "chat:messages", jsonData).Err()
	if err != nil {
		log.Printf("❌ Ошибка публикации в Redis: %v", err)
	}
}

// publishFileEventToRedis публикует файловое событие в Redis
func (m *Manager) publishFileEventToRedis(event FileEventData) {
	if m.RedisClient == nil {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("❌ Ошибка сериализации файлового события: %v", err)
		return
	}

	wsMsg := struct {
		ServerID string `json:"server_id"`
		Data     []byte `json:"data"`
	}{
		ServerID: m.ServerID,
		Data:     data,
	}

	jsonData, _ := json.Marshal(wsMsg)

	err = m.RedisClient.Publish(context.Background(), "chat:files", jsonData).Err()
	if err != nil {
		log.Printf("❌ Ошибка публикации в Redis: %v", err)
	}
}

// subscribeToRedis подписывается на каналы Redis
func (m *Manager) subscribeToRedis(ctx context.Context) {
	m.PubSub = m.RedisClient.Subscribe(ctx, "chat:messages", "chat:files")

	go func() {
		for {
			msg, err := m.PubSub.ReceiveMessage(ctx)
			if err != nil {
				log.Printf("❌ Ошибка Redis Pub/Sub: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			m.RedisMsgCh <- []byte(msg.Payload)
		}
	}()

	go m.processRedisMessages(ctx)
}

// processRedisMessages обрабатывает сообщения из Redis
func (m *Manager) processRedisMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg := <-m.RedisMsgCh:
			var wsMsg struct {
				ServerID string          `json:"server_id"`
				Data     json.RawMessage `json:"data"`
			}

			if err := json.Unmarshal(redisMsg, &wsMsg); err != nil {
				log.Printf("❌ Ошибка парсинга Redis сообщения: %v", err)
				continue
			}

			if wsMsg.ServerID == m.ServerID {
				continue
			}

			var message models.Message
			if err := json.Unmarshal(wsMsg.Data, &message); err == nil {
				m.broadcastToLocalClients(message)
				continue
			}

			var fileEvent FileEventData
			if err := json.Unmarshal(wsMsg.Data, &fileEvent); err == nil {
				m.broadcastFileEventToLocalClients(fileEvent)
				continue
			}
		}
	}
}

// GetStats возвращает статистику менеджера
func (m *Manager) GetStats() map[string]interface{} {
	m.ClientsMu.RLock()
	m.OnlineMu.RLock()
	defer m.ClientsMu.RUnlock()
	defer m.OnlineMu.RUnlock()

	stats := map[string]interface{}{
		"server_id":              m.ServerID,
		"clients_count":          len(m.Clients),
		"online_count":           len(m.OnlineUsers),
		"redis_enabled":          m.RedisClient != nil,
		"broadcast_queue_size":   len(m.Broadcast),
		"file_events_queue_size": len(m.FileEvents),
	}

	return stats
}
