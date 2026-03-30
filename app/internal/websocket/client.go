package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"pigeongram/app/pkg/models"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID            string
	Username      string
	Conn          *websocket.Conn
	Send          chan models.Message // для текстовых сообщений
	SendFileEvent chan interface{}    // для файловых событий
	SendOnline    chan interface{}    // для онлайн статусов
	Manager       *Manager
	ConnectedAt   time.Time
}

func NewClient(conn *websocket.Conn, username string, manager *Manager) *Client {
	return &Client{
		ID:            generateClientID(),
		Username:      username,
		Conn:          conn,
		Send:          make(chan models.Message, 256),
		SendFileEvent: make(chan interface{}, 32),
		SendOnline:    make(chan interface{}, 8),
		Manager:       manager,
		ConnectedAt:   time.Now(),
	}
}

// ReadPump читает сообщения от клиента
func (c *Client) ReadPump() {
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(4096)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var rawMsg json.RawMessage
		err := c.Conn.ReadJSON(&rawMsg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ [CLIENT %s] Ошибка чтения: %v", c.Username, err)
			}
			break
		}

		// Логируем сырое сообщение для отладки
		log.Printf("📨 [CLIENT %s] Сырое сообщение: %s", c.Username, string(rawMsg))

		// Пробуем распарсить как разные форматы
		var text string
		var username string = c.Username // По умолчанию используем имя текущего клиента

		// Формат 1: просто строка
		var simpleString string
		if err := json.Unmarshal(rawMsg, &simpleString); err == nil && simpleString != "" {
			text = simpleString
			log.Printf("📨 Формат 1 (строка): %s", text)
		}

		// Формат 2: объект с полем text
		if text == "" {
			var textObj struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(rawMsg, &textObj); err == nil && textObj.Text != "" {
				text = textObj.Text
				log.Printf("📨 Формат 2 (объект с text): %s", text)
			}
		}

		// Формат 3: объект с type и data
		if text == "" {
			var typedMsg struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rawMsg, &typedMsg); err == nil {
				if typedMsg.Type == "chat" || typedMsg.Type == "message" {
					// Пробуем распарсить data как объект с text
					var dataObj struct {
						Text string `json:"text"`
					}
					if err := json.Unmarshal(typedMsg.Data, &dataObj); err == nil && dataObj.Text != "" {
						text = dataObj.Text
						log.Printf("📨 Формат 3 (type/data): %s", text)
					}

					// Пробуем распарсить data как строку
					if text == "" {
						var dataStr string
						if err := json.Unmarshal(typedMsg.Data, &dataStr); err == nil && dataStr != "" {
							text = dataStr
							log.Printf("📨 Формат 3b (data как строка): %s", text)
						}
					}
				}
			}
		}

		// Формат 4: объект с username и text
		if text == "" {
			var msg struct {
				Username string `json:"username"`
				Text     string `json:"text"`
			}
			if err := json.Unmarshal(rawMsg, &msg); err == nil {
				if msg.Text != "" {
					text = msg.Text
					if msg.Username != "" {
						username = msg.Username
					}
					log.Printf("📨 Формат 4 (username/text): %s от %s", text, username)
				}
			}
		}

		// Валидация: сообщение должно иметь текст
		if text == "" {
			log.Printf("⚠️ [CLIENT %s] Пустое сообщение проигнорировано: %s", c.Username, string(rawMsg))
			continue
		}

		// Дополнительная проверка: текст не должен состоять только из пробелов
		text = strings.TrimSpace(text)
		if text == "" {
			log.Printf("⚠️ [CLIENT %s] Сообщение только из пробелов проигнорировано", c.Username)
			continue
		}

		// Проверка длины (защита от слишком длинных сообщений)
		if len(text) > 10000 {
			text = text[:10000] + "... (обрезано)"
			log.Printf("⚠️ [CLIENT %s] Сообщение слишком длинное, обрезано", c.Username)
		}

		log.Printf("💬 [CLIENT %s] ОТПРАВКА сообщения: %s от %s", c.Username, text, username)

		// Создаем сообщение
		message := models.Message{
			Username:  username,
			Text:      text,
			Timestamp: time.Now(),
		}

		// Отправляем в менеджер для обработки
		select {
		case c.Manager.Broadcast <- message:
			// Успешно отправлено
		default:
			log.Printf("⚠️ [CLIENT %s] Канал broadcast переполнен", c.Username)
		}
	}
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Отправляем текстовое сообщение
			c.writeJSON(message)

		case fileEvent, ok := <-c.SendFileEvent:
			if !ok {
				return
			}
			// Отправляем файловое событие
			c.writeJSON(fileEvent)

		case onlineData, ok := <-c.SendOnline:
			if !ok {
				return
			}
			// Отправляем онлайн статус
			c.writeJSON(onlineData)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// writeJSON отправляет данные в формате JSON
func (c *Client) writeJSON(data interface{}) {
	c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := c.Conn.WriteJSON(data); err != nil {
		log.Printf("❌ [CLIENT %s] Ошибка отправки: %v", c.Username, err)
	}
}

// generateClientID генерирует уникальный ID клиента
func generateClientID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
