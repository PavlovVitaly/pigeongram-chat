package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"pigeongram/repository/cache"
	"time"
)

var startTime = time.Now()

// getCacheType возвращает тип используемого кэша
func getCacheType() string {
	if messageCache == nil {
		return "none"
	}

	// Проверяем тип кэша через интерфейс
	switch messageCache.(type) {
	case *cache.RedisCache:
		return "redis"
	case *cache.MemoryCache:
		return "memory"
	default:
		return "unknown"
	}
}

// DebugStatsHandler возвращает статистику для отладки
func DebugStatsHandler(w http.ResponseWriter, r *http.Request) {
	if wsManager == nil {
		http.Error(w, "WebSocket manager not initialized", http.StatusServiceUnavailable)
		return
	}

	// Получаем статистику от менеджера
	managerStats := wsManager.GetStats()

	stats := map[string]interface{}{
		"server_id":     managerStats["server_id"],
		"clients_count": managerStats["clients_count"],
		"clients":       managerStats["clients"],
		"redis_enabled": wsManager.RedisClient != nil,
		"cache_type":    getCacheType(),
		"uptime":        time.Since(startTime).String(),
		"start_time":    startTime.Format(time.RFC3339),
		"current_time":  time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HealthCheckHandler для проверки работоспособности
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
		"uptime": time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// MetricsHandler для сбора метрик (если нужно)
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if wsManager == nil {
		http.Error(w, "WebSocket manager not initialized", http.StatusServiceUnavailable)
		return
	}

	stats := wsManager.GetStats()

	metrics := map[string]interface{}{
		"pigeongram_clients_total":   stats["clients_count"],
		"pigeongram_uptime_seconds":  time.Since(startTime).Seconds(),
		"pigeongram_cache_type":      getCacheType(),
		"pigeongram_redis_connected": wsManager.RedisClient != nil,
	}

	// Формат для Prometheus (опционально)
	w.Header().Set("Content-Type", "text/plain")
	for key, value := range metrics {
		fmt.Fprintf(w, "%s %v\n", key, value)
	}
}
