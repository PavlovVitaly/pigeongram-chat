package config

import (
	"time"
)

// RedisConfigReader читает конфигурацию Redis
type RedisConfigReader struct{}

func NewRedisConfigReader() *RedisConfigReader {
	return &RedisConfigReader{}
}

// Load загружает конфигурацию из переменных окружения
func (r *RedisConfigReader) Load() (host string, port int, password string, db int, ttl time.Duration) {
	host = GetEnv("REDIS_HOST", "localhost")
	port = GetEnvAsInt("REDIS_PORT", 6379)
	password = GetEnv("REDIS_PASSWORD", "redis_secret")
	db = GetEnvAsInt("REDIS_DB", 0)
	ttl = GetEnvAsDuration("REDIS_TTL", 5*time.Minute)

	return
}

// GetCachePrefix возвращает префикс для ключей
func (r *RedisConfigReader) GetCachePrefix() string {
	return GetEnv("REDIS_PREFIX", "pigeongram")
}

// IsEnabled проверяет, включен ли Redis
func (r *RedisConfigReader) IsEnabled() bool {
	return GetEnvAsBool("REDIS_ENABLED", true)
}
