package models

import "time"

// Message - общий тип сообщения (DTO)
type Message struct {
	ID        uint      `json:"id,omitempty"`
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Edited    bool      `json:"edited,omitempty"`
}

// User - общий тип пользователя (DTO)
type User struct {
	Username string `json:"username"`
	Password string `json:"-"` // не отправляем в JSON
}

// Session - общий тип сессии (DTO)
type Session struct {
	Username string `json:"username"`
}
