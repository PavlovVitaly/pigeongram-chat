package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"pigeongram/pkg/models"
)

// Client представляет подключенного клиента
type Client struct {
	ID          string
	Username    string
	Conn        *websocket.Conn
	Send        chan models.Message
	Manager     *Manager
	ConnectedAt time.Time
}

// NewClient создает нового клиента
func NewClient(conn *websocket.Conn, username string, manager *Manager) *Client {
	return &Client{
		ID:          generateClientID(),
		Username:    username,
		Conn:        conn,
		Send:        make(chan models.Message, 256),
		Manager:     manager,
		ConnectedAt: time.Now(),
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
		var msg struct {
			Text string `json:"text"`
		}
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		if msg.Text != "" {
			// Создаем сообщение
			message := models.Message{
				Username:  c.Username,
				Text:      msg.Text,
				Timestamp: time.Now(),
			}

			// Отправляем в менеджер для обработки
			c.Manager.Broadcast <- message
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

			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("❌ Ошибка сериализации: %v", err)
				continue
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// generateClientID генерирует уникальный ID клиента
func generateClientID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
