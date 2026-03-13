package cache

import (
	"context"
	"pigeongram/pkg/models" // ← импортируем общие типы
)

// Cache интерфейс для кэширования
type Cache interface {
	// Сообщения
	GetRecentMessages(ctx context.Context) ([]models.Message, error)
	SetRecentMessages(ctx context.Context, messages []models.Message) error
	AddMessage(ctx context.Context, message models.Message) error
	InvalidateMessages(ctx context.Context) error

	// Пользователи (для будущего использования)
	GetUser(ctx context.Context, username string) (*models.User, error)
	SetUser(ctx context.Context, username string, user *models.User) error
	InvalidateUser(ctx context.Context, username string) error

	// Сессии (для будущего использования)
	GetSession(ctx context.Context, sessionID string) (*models.Session, error)
	SetSession(ctx context.Context, sessionID string, session *models.Session) error
	InvalidateSession(ctx context.Context, sessionID string) error
}

// CacheStats для мониторинга
type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int64
}
