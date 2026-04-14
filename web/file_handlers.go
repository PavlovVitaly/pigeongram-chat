package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"pigeongram/internal/metrics"
	"pigeongram/internal/storage"
	"pigeongram/internal/websocket"
)

type FileHandler struct {
	storage   *storage.MinIOClient
	wsManager *websocket.Manager // 👈 Добавляем ссылку на WebSocket менеджер
}

func NewFileHandler(storage *storage.MinIOClient, wsManager *websocket.Manager) *FileHandler {
	return &FileHandler{
		storage:   storage,
		wsManager: wsManager,
	}
}

// FilePage - страница хранилища файлов
func (h *FileHandler) FilePage(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		chatID = "general" // общий чат по умолчанию
	}

	data := map[string]interface{}{
		"Username": username,
		"ChatID":   chatID,
	}

	renderTemplate(w, "files.html", data)
}

// RequestUpload - клиент запрашивает URL для загрузки
func (h *FileHandler) RequestUpload(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	filename := r.URL.Query().Get("filename")

	if chatID == "" || filename == "" {
		http.Error(w, "chat_id and filename required", http.StatusBadRequest)
		return
	}

	log.Printf("📤 [UPLOAD] Запрос на загрузку: user=%s, file=%s", username, filename)

	// 👇 Улучшенная очистка имени файла
	// Убираем path traversal
	filename = filepath.Base(filename)

	// Заменяем проблемные символы на безопасные
	filename = strings.ReplaceAll(filename, "'", "_")  // апостроф
	filename = strings.ReplaceAll(filename, "\"", "_") // кавычки
	filename = strings.ReplaceAll(filename, "`", "_")  // бэктик
	filename = strings.ReplaceAll(filename, "$", "_")  // доллар
	filename = strings.ReplaceAll(filename, "&", "_")  // амперсанд
	filename = strings.ReplaceAll(filename, "|", "_")  // пайп
	filename = strings.ReplaceAll(filename, ";", "_")  // точка с запятой
	filename = strings.ReplaceAll(filename, "(", "_")  // скобки
	filename = strings.ReplaceAll(filename, ")", "_")
	filename = strings.ReplaceAll(filename, "[", "_")
	filename = strings.ReplaceAll(filename, "]", "_")
	filename = strings.ReplaceAll(filename, "{", "_")
	filename = strings.ReplaceAll(filename, "}", "_")

	// Ограничиваем длину
	if len(filename) > 200 {
		ext := filepath.Ext(filename)
		name := filename[:200-len(ext)]
		filename = name + ext
	}

	log.Printf("📤 [UPLOAD] Очищенное имя: %s", filename)

	url, formData, err := h.storage.GenerateUploadURL(r.Context(), chatID, username, filename)
	if err != nil {
		log.Printf("❌ [UPLOAD] Ошибка: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"upload_url":    url,
		"form_data":     formData,
		"safe_filename": filename,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UploadComplete - уведомление о завершении загрузки
func (h *FileHandler) UploadComplete(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ChatID   string `json:"chat_id"`
		Filename string `json:"filename"`
		Key      string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("📥 [UPLOAD_COMPLETE] Получено уведомление: chat=%s, file=%s, key=%s, user=%s",
		req.ChatID, req.Filename, req.Key, username)

	// Очищаем ключ от лишних частей (если пришел полный URL)
	cleanKey := req.Key
	if strings.Contains(cleanKey, "/"+h.storage.GetBucketName()+"/") {
		// Извлекаем ключ из полного URL
		parts := strings.Split(cleanKey, "/"+h.storage.GetBucketName()+"/")
		if len(parts) > 1 {
			cleanKey = parts[1]
			log.Printf("🔧 [UPLOAD_COMPLETE] Очищенный ключ: %s", cleanKey)
		}
	}

	// Получаем информацию о загруженном файле
	fileInfo, err := h.storage.GetFileInfo(r.Context(), req.ChatID, cleanKey)
	if err != nil {
		log.Printf("❌ [UPLOAD_COMPLETE] Ошибка получения информации о файле: %v", err)

		// Пробуем создать файлInfo вручную, если MinIO не отвечает
		fileInfo = &storage.FileInfo{
			Name:       req.Filename,
			Size:       0,
			UploadedAt: time.Now(),
			UploaderID: username,
			ChatID:     req.ChatID,
			Key:        cleanKey,
		}
		log.Printf("⚠️ [UPLOAD_COMPLETE] Используем ручное создание FileInfo")
	} else {
		log.Printf("✅ [UPLOAD_COMPLETE] Файл найден в MinIO: %s, размер: %d bytes",
			fileInfo.Name, fileInfo.Size)
	}

	// Отправляем событие через WebSocket
	if h.wsManager != nil {
		log.Printf("📤 [UPLOAD_COMPLETE] Отправляем событие в WebSocket")

		event := websocket.FileEventData{
			EventType: "upload",
			ChatID:    req.ChatID,
			File:      fileInfo,
			Username:  username,
			Owner:     username,
			Timestamp: time.Now(),
		}

		select {
		case h.wsManager.FileEvents <- event:
			log.Printf("✅ [UPLOAD_COMPLETE] Событие успешно отправлено в канал FileEvents")
		default:
			log.Printf("❌ [UPLOAD_COMPLETE] Канал FileEvents переполнен!")
		}
	} else {
		log.Printf("❌ [UPLOAD_COMPLETE] wsManager = nil!")
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	metrics.FilesUploadedTotal.Inc()
	metrics.FilesSizeBytes.Add(float64(fileInfo.Size))
}

// ListFiles - список файлов в чате
func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		http.Error(w, "chat_id required", http.StatusBadRequest)
		return
	}

	files, err := h.storage.ListFiles(r.Context(), chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Если files == nil, возвращаем пустой массив
	if files == nil {
		files = make([]storage.FileInfo, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetDownloadURL - получить ссылку на скачивание
func (h *FileHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		log.Printf("❌ [DOWNLOAD] Неавторизованный доступ")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	objectKey := r.URL.Query().Get("key")

	log.Printf("📥 [DOWNLOAD] Запрос: user=%s, chat=%s, key=%s",
		username, chatID, objectKey)

	if chatID == "" || objectKey == "" {
		log.Printf("❌ [DOWNLOAD] Отсутствуют параметры")
		http.Error(w, "chat_id and key required", http.StatusBadRequest)
		return
	}

	// Декодируем URL-encoded ключ
	decodedKey, err := url.QueryUnescape(objectKey)
	if err != nil {
		decodedKey = objectKey
	}
	log.Printf("🔑 [DOWNLOAD] Декодированный ключ: %s", decodedKey)

	url, err := h.storage.GenerateDownloadURL(r.Context(), chatID, decodedKey)
	if err != nil {
		log.Printf("❌ [DOWNLOAD] Ошибка: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [DOWNLOAD] URL сгенерирован: %s", url)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"download_url": url,
	})
}

// DeleteFile - удаление файла
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		log.Printf("❌ [DELETE] Неавторизованный доступ")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ChatID string `json:"chat_id"`
		Key    string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [DELETE] Ошибка парсинга: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("🗑️ [DELETE] Запрос: user=%s, chat=%s, key=%s",
		username, req.ChatID, req.Key)

	if req.ChatID == "" || req.Key == "" {
		log.Printf("❌ [DELETE] Отсутствуют параметры")
		http.Error(w, "chat_id and key required", http.StatusBadRequest)
		return
	}

	// Декодируем URL-encoded ключ
	decodedKey, err := url.QueryUnescape(req.Key)
	if err != nil {
		decodedKey = req.Key
	}
	log.Printf("🔑 [DELETE] Декодированный ключ: %s", decodedKey)

	// Проверяем, что файл принадлежит чату
	expectedPrefix := fmt.Sprintf("chat-%s/", req.ChatID)
	if !strings.HasPrefix(decodedKey, expectedPrefix) {
		log.Printf("❌ [DELETE] Файл не принадлежит чату: %s", decodedKey)
		http.Error(w, "File does not belong to this chat", http.StatusForbidden)
		return
	}

	// Извлекаем владельца
	parts := strings.Split(decodedKey, "/")
	if len(parts) < 2 {
		log.Printf("❌ [DELETE] Неверный формат ключа: %s", decodedKey)
		http.Error(w, "Invalid file key format", http.StatusBadRequest)
		return
	}

	fileOwner := parts[1]
	log.Printf("👤 [DELETE] Владелец: %s, запросил: %s", fileOwner, username)

	if fileOwner != username {
		log.Printf("⛔ [DELETE] Доступ запрещен")
		http.Error(w, "You can only delete your own files", http.StatusForbidden)
		return
	}

	err = h.storage.DeleteFile(r.Context(), req.ChatID, decodedKey, username)
	if err != nil {
		log.Printf("❌ [DELETE] Ошибка: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [DELETE] Файл удален: %s", decodedKey)

	// Отправляем событие
	if h.wsManager != nil {
		event := websocket.FileEventData{
			EventType: "delete",
			ChatID:    req.ChatID,
			File:      map[string]string{"key": decodedKey},
			Username:  username,
			Owner:     username,
			Timestamp: time.Now(),
		}

		select {
		case h.wsManager.FileEvents <- event:
			log.Printf("📤 [DELETE] Событие отправлено")
		default:
			log.Printf("⚠️ [DELETE] Канал переполнен")
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "File deleted",
	})
}

// DebugLastMessages - показывает последние сообщения в системе (для отладки)
func (h *FileHandler) DebugLastMessages(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Только для администратора (временно)
	if username != "admin" && username != "test" {
		http.Error(w, "Доступ запрещен", http.StatusForbidden)
		return
	}

	ctx := context.Background()

	// Получаем последние сообщения из репозитория
	// Для этого нужно, чтобы в messageRepo был метод GetRecent
	if msgRepo == nil {
		http.Error(w, "Message repository not available", http.StatusInternalServerError)
		return
	}

	// Получаем последние 50 сообщений
	messages, err := msgRepo.GetRecent(ctx, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Конвертируем в DTO для вывода
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		result = append(result, map[string]interface{}{
			"id":        msg.ID,
			"username":  msg.User.Username,
			"text":      msg.Content,
			"timestamp": msg.Timestamp,
			"user_id":   msg.UserID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"count":    len(result),
		"messages": result,
	})
}

// DebugTestMinIO - тестирование MinIO
func (h *FileHandler) DebugTestMinIO(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()

	// Проверяем список bucket'ов
	buckets, err := h.storage.ListBuckets(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Проверяем файлы в общем чате
	files, err := h.storage.ListFiles(ctx, "general")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := map[string]interface{}{
		"status":           "ok",
		"buckets":          buckets,
		"bucket_name":      h.storage.GetBucketName(),
		"files_in_general": files,
		"files_count":      len(files),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// DebugFileExists - проверка существования файла
func (h *FileHandler) DebugFileExists(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Пробуем получить информацию о файле
	info, err := h.storage.GetFileInfo(ctx, "general", key)

	result := map[string]interface{}{
		"key":    key,
		"exists": err == nil,
	}

	if err == nil {
		result["info"] = info
	} else {
		result["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ProxyDownload - проксирует загрузку файла через приложение (обходит проблему с подписью)
func (h *FileHandler) ProxyDownload(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	objectKey := r.URL.Query().Get("key")

	if chatID == "" || objectKey == "" {
		http.Error(w, "chat_id and key required", http.StatusBadRequest)
		return
	}

	decodedKey, _ := url.QueryUnescape(objectKey)

	log.Printf("📥 [DOWNLOAD] Прокси-скачивание: user=%s, chat=%s, key=%s", username, chatID, decodedKey)

	// Проверка принадлежности чату
	expectedPrefix := fmt.Sprintf("chat-%s/", chatID)
	if !strings.HasPrefix(decodedKey, expectedPrefix) {
		http.Error(w, "File does not belong to this chat", http.StatusForbidden)
		return
	}

	// Получаем объект из MinIO
	obj, err := h.storage.GetObject(r.Context(), decodedKey)
	if err != nil {
		log.Printf("❌ [DOWNLOAD] Ошибка получения объекта: %v", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer obj.Close()

	// Получаем статистику файла
	stat, err := obj.Stat()

	// Формируем имя файла
	filename := filepath.Base(decodedKey)
	if idx := strings.Index(filename, "-"); idx > 0 {
		if strings.Trim(filename[:idx], "0123456789") == "" {
			filename = filename[idx+1:]
		}
	}
	filename = strings.ReplaceAll(filename, "_", " ")

	// Устанавливаем заголовки
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	if err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
		log.Printf("✅ [DOWNLOAD] Отправка: %s (размер: %d)", filename, stat.Size)
	} else {
		log.Printf("✅ [DOWNLOAD] Отправка: %s (размер неизвестен)", filename)
	}

	// Отправляем файл
	http.ServeContent(w, r, filename, time.Now(), obj)
}
