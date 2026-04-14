package websocket

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// ServerRegistry регистрирует и отслеживает активные серверы
type ServerRegistry struct {
	RedisClient *redis.Client
	ServerID    string
	TTL         time.Duration
}

// NewServerRegistry создает новый реестр серверов
func NewServerRegistry(redisClient *redis.Client, serverID string) *ServerRegistry {
	return &ServerRegistry{
		RedisClient: redisClient,
		ServerID:    serverID,
		TTL:         10 * time.Second,
	}
}

// Register регистрирует сервер в Redis
func (r *ServerRegistry) Register(ctx context.Context) error {
	key := fmt.Sprintf("server:%s", r.ServerID)

	serverInfo := map[string]interface{}{
		"server_id":  r.ServerID,
		"started_at": time.Now().Format(time.RFC3339),
		"last_seen":  time.Now().Format(time.RFC3339),
		"status":     "active",
	}

	// Сохраняем информацию о сервере
	if err := r.RedisClient.HSet(ctx, key, serverInfo).Err(); err != nil {
		return err
	}

	// Устанавливаем TTL
	if err := r.RedisClient.Expire(ctx, key, r.TTL).Err(); err != nil {
		return err
	}

	log.Printf("✅ Сервер %s зарегистрирован в Redis", r.ServerID)

	// Запускаем обновление регистрации
	go r.heartbeat(ctx)

	return nil
}

// heartbeat периодически обновляет регистрацию сервера
func (r *ServerRegistry) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(r.TTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			key := fmt.Sprintf("server:%s", r.ServerID)

			r.RedisClient.HSet(ctx, key, "last_seen", time.Now().Format(time.RFC3339))
			r.RedisClient.Expire(ctx, key, r.TTL)
		}
	}
}

// Unregister удаляет регистрацию сервера
func (r *ServerRegistry) Unregister(ctx context.Context) error {
	key := fmt.Sprintf("server:%s", r.ServerID)
	return r.RedisClient.Del(ctx, key).Err()
}

// GetActiveServers возвращает список активных серверов
func (r *ServerRegistry) GetActiveServers(ctx context.Context) ([]string, error) {
	keys, err := r.RedisClient.Keys(ctx, "server:*").Result()
	if err != nil {
		return nil, err
	}

	servers := make([]string, 0, len(keys))
	for _, key := range keys {
		serverID := key[len("server:"):]

		// Проверяем, жив ли сервер
		exists, err := r.RedisClient.Exists(ctx, key).Result()
		if err == nil && exists > 0 {
			servers = append(servers, serverID)
		}
	}

	return servers, nil
}
