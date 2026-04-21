package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"pigeongram-chat/internal/storage"
	"pigeongram-chat/internal/websocket"
	"pigeongram-chat/repository/cache"
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

// TestFileNotification - тестовый эндпоинт для проверки уведомлений
func (h *FileHandler) TestFileNotification(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		chatID = "general"
	}

	// Создаем тестовое событие
	testFile := storage.FileInfo{
		Name:       "test-file.txt",
		Size:       1024,
		UploadedAt: time.Now(),
		UploaderID: username,
		ChatID:     chatID,
		Key:        "chat-general/testuser/test-file.txt",
	}

	event := websocket.FileEventData{
		EventType: "upload",
		ChatID:    chatID,
		File:      testFile,
		Username:  username,
		Timestamp: time.Now(),
	}

	// Отправляем через WebSocket
	if h.wsManager != nil {
		select {
		case h.wsManager.FileEvents <- event:
			log.Printf("📤 Тестовое событие отправлено для чата %s", chatID)
		default:
			log.Printf("⚠️ Канал файловых событий переполнен")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "test notification sent",
		"chat_id":  chatID,
		"username": username,
	})
}

// DebugWebSocket - информация о WebSocket соединениях
func (h *FileHandler) DebugWebSocket(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.wsManager == nil {
		http.Error(w, "WebSocket manager not initialized", http.StatusInternalServerError)
		return
	}

	// Получаем статистику из менеджера
	stats := map[string]interface{}{
		"server_id":       h.wsManager.ServerID,
		"clients_count":   len(h.wsManager.Clients),
		"redis_connected": h.wsManager.RedisClient != nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// DebugSendTestEvent - отправка тестового события конкретному клиенту
func (h *FileHandler) DebugSendTestEvent(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	targetUser := r.URL.Query().Get("user")
	chatID := r.URL.Query().Get("chat_id")

	if targetUser == "" {
		targetUser = username
	}
	if chatID == "" {
		chatID = "general"
	}

	if h.wsManager == nil {
		http.Error(w, "WebSocket manager not initialized", http.StatusInternalServerError)
		return
	}

	// Создаем тестовое событие
	testFile := storage.FileInfo{
		Name:       "test-file.txt",
		Size:       1024,
		UploadedAt: time.Now(),
		UploaderID: "system",
		ChatID:     chatID,
		Key:        "chat-general/system/test-file.txt",
	}

	event := websocket.FileEventData{
		EventType: "upload",
		ChatID:    chatID,
		File:      testFile,
		Username:  "system",
		Timestamp: time.Now(),
	}

	// Ищем клиента с указанным username
	h.wsManager.ClientsMu.RLock()
	defer h.wsManager.ClientsMu.RUnlock()

	sent := false
	for client := range h.wsManager.Clients {
		if client.Username == targetUser {
			select {
			case client.SendFileEvent <- map[string]interface{}{
				"type": "file",
				"data": event,
			}:
				sent = true
				log.Printf("📤 Тестовое событие отправлено пользователю %s", targetUser)
			default:
				log.Printf("⚠️ Канал клиента %s переполнен", targetUser)
			}
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"sent":        sent,
		"target_user": targetUser,
		"chat_id":     chatID,
	})
}

// DebugCheckFile - проверка доступа к файлу
func (h *FileHandler) DebugCheckFile(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	objectKey := r.URL.Query().Get("key")
	if objectKey == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		chatID = "general"
	}

	log.Printf("🔍 [DEBUG] Проверка файла: key=%s", objectKey)

	// Пробуем получить информацию
	fileInfo, err := h.storage.GetFileInfo(r.Context(), chatID, objectKey)

	result := map[string]interface{}{
		"key":     objectKey,
		"success": err == nil,
	}

	if err == nil {
		result["file_info"] = fileInfo
	} else {
		result["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
