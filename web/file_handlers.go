package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"pigeongram/internal/storage"
	"pigeongram/internal/websocket" // 👈 ДОБАВЛЯЕМ ЭТОТ ИМПОРТ
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

	// Очищаем имя файла от path traversal
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	filename = strings.ReplaceAll(filename, "..", "")

	url, formData, err := h.storage.GenerateUploadURL(r.Context(), chatID, username, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"upload_url": url,
		"form_data":  formData,
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := r.URL.Query().Get("chat_id")
	objectKey := r.URL.Query().Get("key")

	if chatID == "" || objectKey == "" {
		http.Error(w, "chat_id and key required", http.StatusBadRequest)
		return
	}

	url, err := h.storage.GenerateDownloadURL(r.Context(), chatID, objectKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"download_url": url,
	})
}

// DeleteFile - удаление файла
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ChatID string `json:"chat_id"`
		Key    string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.storage.DeleteFile(r.Context(), req.ChatID, req.Key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Отправляем событие о удалении
	if h.wsManager != nil {
		event := websocket.FileEventData{
			EventType: "delete",
			ChatID:    req.ChatID,
			File:      map[string]string{"key": req.Key},
			Username:  username,
			Timestamp: time.Now(),
		}

		h.wsManager.FileEvents <- event
		log.Printf("🗑️ Событие об удалении файла отправлено: %s удалил файл", username)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
