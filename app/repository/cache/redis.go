package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"pigeongram/app/pkg/models"

	"github.com/go-redis/redis/v8"
	// добавляем для использования GetEnvAsDuration если нужно
)

// RedisCache реализует кэш в Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// RedisConfig конфигурация для Redis
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	TTL      time.Duration
	Prefix   string
}

// NewRedisCache создает новый Redis кэш
func NewRedisCache(cfg RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("❌ Ошибка подключения к Redis: %w", err)
	}

	log.Printf("✅ Redis подключен: %s:%d (БД: %d)", cfg.Host, cfg.Port, cfg.DB)

	return &RedisCache{
		client: client,
		ttl:    cfg.TTL,
		prefix: cfg.Prefix,
	}, nil
}

// ключи для Redis
func (r *RedisCache) keyMessages() string {
	return fmt.Sprintf("%s:recent:messages", r.prefix)
}

func (r *RedisCache) keyUser(username string) string {
	return fmt.Sprintf("%s:user:%s", r.prefix, username)
}

func (r *RedisCache) keySession(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", r.prefix, sessionID)
}

// ========== РАБОТА С СООБЩЕНИЯМИ ==========

func (r *RedisCache) GetRecentMessages(ctx context.Context) ([]models.Message, error) {
	data, err := r.client.Get(ctx, r.keyMessages()).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // ключ не найден
		}
		return nil, fmt.Errorf("ошибка получения сообщений из Redis: %w", err)
	}

	var messages []models.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("ошибка десериализации сообщений: %w", err)
	}

	log.Printf("📦 Загружено %d сообщений из Redis", len(messages))
	return messages, nil
}

func (r *RedisCache) SetRecentMessages(ctx context.Context, messages []models.Message) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("ошибка сериализации сообщений: %w", err)
	}

	if err := r.client.Set(ctx, r.keyMessages(), data, r.ttl).Err(); err != nil {
		return fmt.Errorf("ошибка сохранения в Redis: %w", err)
	}

	log.Printf("📦 Сохранено %d сообщений в Redis (TTL: %v)", len(messages), r.ttl)
	return nil
}

// AddMessage добавляет одно сообщение в кэш
func (r *RedisCache) AddMessage(ctx context.Context, message models.Message) error {
	// Получаем текущие сообщения
	messages, err := r.GetRecentMessages(ctx)
	if err != nil {
		return err
	}

	// Если сообщений нет, создаем новый слайс
	if messages == nil {
		messages = []models.Message{message}
	} else {
		// Проверяем, нет ли уже такого сообщения (по времени)
		for _, msg := range messages {
			if msg.Username == message.Username &&
				msg.Text == message.Text &&
				msg.Timestamp.Equal(message.Timestamp) {
				log.Printf("⚠️ Redis: дубликат сообщения от %s", message.Username)
				return nil // игнорируем дубликаты
			}
		}

		// Добавляем новое сообщение
		messages = append(messages, message)

		// Оставляем только последние 100
		if len(messages) > 100 {
			messages = messages[len(messages)-100:]
		}
	}

	// Сохраняем обратно
	return r.SetRecentMessages(ctx, messages)
}

func (r *RedisCache) InvalidateMessages(ctx context.Context) error {
	return r.client.Del(ctx, r.keyMessages()).Err()
}

// ========== РАБОТА С ПОЛЬЗОВАТЕЛЯМИ ==========

func (r *RedisCache) GetUser(ctx context.Context, username string) (*models.User, error) {
	data, err := r.client.Get(ctx, r.keyUser(username)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var user models.User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *RedisCache) SetUser(ctx context.Context, username string, user *models.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, r.keyUser(username), data, r.ttl).Err()
}

func (r *RedisCache) InvalidateUser(ctx context.Context, username string) error {
	return r.client.Del(ctx, r.keyUser(username)).Err()
}

// ========== РАБОТА С СЕССИЯМИ ==========

func (r *RedisCache) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	data, err := r.client.Get(ctx, r.keySession(sessionID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *RedisCache) SetSession(ctx context.Context, sessionID string, session *models.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, r.keySession(sessionID), data, r.ttl).Err()
}

func (r *RedisCache) InvalidateSession(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, r.keySession(sessionID)).Err()
}

// ========== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ==========

// Close закрывает соединение с Redis
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// FlushAll очищает все кэш (осторожно!)
func (r *RedisCache) FlushAll(ctx context.Context) error {
	return r.client.FlushAll(ctx).Err()
}

// Stats возвращает статистику Redis
func (r *RedisCache) Stats(ctx context.Context) map[string]string {
	info, err := r.client.Info(ctx).Result()
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	return map[string]string{"info": info}
}
