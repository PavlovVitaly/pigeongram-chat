package cache

import (
	"context"
	"log"
	"sync"
	"time"

	"pigeongram/pkg/models" // ← импортируем общие типы
)

// MemoryCache реализует кэш в памяти
type MemoryCache struct {
	// Кэш сообщений
	messages     []models.Message
	messagesTime time.Time
	messagesMu   sync.RWMutex

	// Кэш пользователей
	users   map[string]cachedUser
	usersMu sync.RWMutex

	// Кэш сессий
	sessions   map[string]cachedSession
	sessionsMu sync.RWMutex

	// Статистика
	stats   CacheStats
	statsMu sync.RWMutex

	// Настройки
	ttl time.Duration // время жизни кэша
}

type cachedUser struct {
	user      *models.User
	timestamp time.Time
}

type cachedSession struct {
	session   *models.Session
	timestamp time.Time
}

// NewMemoryCache создает новый кэш в памяти
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		users:    make(map[string]cachedUser),
		sessions: make(map[string]cachedSession),
		ttl:      ttl,
	}

	// Запускаем горутину для очистки устаревших записей
	go cache.cleanupLoop()

	return cache
}

// ========== РАБОТА С СООБЩЕНИЯМИ ==========

// GetRecentMessages возвращает сообщения из кэша
func (c *MemoryCache) GetRecentMessages(ctx context.Context) ([]models.Message, error) {
	c.messagesMu.RLock()
	defer c.messagesMu.RUnlock()

	if time.Since(c.messagesTime) > c.ttl {
		c.updateStats(false)
		return nil, nil // кэш устарел
	}

	if len(c.messages) == 0 {
		c.updateStats(false)
		return nil, nil
	}

	c.updateStats(true)

	// Возвращаем копию, чтобы избежать изменений
	result := make([]models.Message, len(c.messages))
	copy(result, c.messages)
	return result, nil
}

// SetRecentMessages сохраняет сообщения в кэш
func (c *MemoryCache) SetRecentMessages(ctx context.Context, messages []models.Message) error {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()

	// Сохраняем копию
	c.messages = make([]models.Message, len(messages))
	copy(c.messages, messages)
	c.messagesTime = time.Now()

	log.Printf("📦 Кэш сообщений обновлен: %d сообщений", len(messages))
	return nil
}

// AddMessage добавляет одно сообщение в кэш
func (c *MemoryCache) AddMessage(ctx context.Context, message models.Message) error {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()

	// Проверяем, не дубликат ли это сообщение
	for _, msg := range c.messages {
		if msg.Username == message.Username &&
			msg.Text == message.Text &&
			msg.Timestamp.Equal(message.Timestamp) {
			log.Printf("⚠️ Попытка добавить дубликат сообщения от %s", message.Username)
			return nil // игнорируем дубликаты
		}
	}

	// Добавляем сообщение в конец (оно самое новое)
	c.messages = append(c.messages, message)

	// Оставляем только последние 100 сообщений
	if len(c.messages) > 100 {
		c.messages = c.messages[len(c.messages)-100:]
	}

	c.messagesTime = time.Now()
	return nil
}

func (c *MemoryCache) InvalidateMessages(ctx context.Context) error {
	c.messagesMu.Lock()
	defer c.messagesMu.Unlock()

	c.messages = nil
	c.messagesTime = time.Time{}

	log.Println("🧹 Кэш сообщений очищен")
	return nil
}

// ========== РАБОТА С ПОЛЬЗОВАТЕЛЯМИ ==========

func (c *MemoryCache) GetUser(ctx context.Context, username string) (*models.User, error) {
	c.usersMu.RLock()
	defer c.usersMu.RUnlock()

	cached, exists := c.users[username]
	if !exists || time.Since(cached.timestamp) > c.ttl {
		c.updateStats(false)
		return nil, nil
	}

	c.updateStats(true)
	return cached.user, nil
}

func (c *MemoryCache) SetUser(ctx context.Context, username string, user *models.User) error {
	c.usersMu.Lock()
	defer c.usersMu.Unlock()

	c.users[username] = cachedUser{
		user:      user,
		timestamp: time.Now(),
	}

	return nil
}

func (c *MemoryCache) InvalidateUser(ctx context.Context, username string) error {
	c.usersMu.Lock()
	defer c.usersMu.Unlock()

	delete(c.users, username)
	return nil
}

// ========== РАБОТА С СЕССИЯМИ ==========

func (c *MemoryCache) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	c.sessionsMu.RLock()
	defer c.sessionsMu.RUnlock()

	cached, exists := c.sessions[sessionID]
	if !exists || time.Since(cached.timestamp) > c.ttl {
		c.updateStats(false)
		return nil, nil
	}

	c.updateStats(true)
	return cached.session, nil
}

func (c *MemoryCache) SetSession(ctx context.Context, sessionID string, session *models.Session) error {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	c.sessions[sessionID] = cachedSession{
		session:   session,
		timestamp: time.Now(),
	}

	return nil
}

func (c *MemoryCache) InvalidateSession(ctx context.Context, sessionID string) error {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()

	delete(c.sessions, sessionID)
	return nil
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

func (c *MemoryCache) updateStats(hit bool) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	if hit {
		c.stats.Hits++
	} else {
		c.stats.Misses++
	}
}

// GetStats возвращает статистику кэша
func (c *MemoryCache) GetStats() CacheStats {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()

	c.messagesMu.RLock()
	size := int64(len(c.messages))
	c.messagesMu.RUnlock()

	stats := c.stats
	stats.Size = size

	return stats
}

// cleanupLoop периодически очищает устаревшие записи
func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *MemoryCache) cleanup() {
	now := time.Now()

	// Очистка пользователей
	c.usersMu.Lock()
	for username, cached := range c.users {
		if now.Sub(cached.timestamp) > c.ttl {
			delete(c.users, username)
		}
	}
	c.usersMu.Unlock()

	// Очистка сессий
	c.sessionsMu.Lock()
	for sessionID, cached := range c.sessions {
		if now.Sub(cached.timestamp) > c.ttl {
			delete(c.sessions, sessionID)
		}
	}
	c.sessionsMu.Unlock()

	// Очистка сообщений, если устарели
	c.messagesMu.RLock()
	oldTime := c.messagesTime
	c.messagesMu.RUnlock()

	if now.Sub(oldTime) > c.ttl {
		c.InvalidateMessages(context.Background())
	}
}
