package models

import (
	"time"

	dto "pigeongram/app/pkg/models" // алиас для DTO

	"gorm.io/gorm"
)

// UserEntity - модель пользователя для БД
type UserEntity struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"`
	LastSeen  time.Time      `json:"lastSeen"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName задает имя таблицы
func (UserEntity) TableName() string {
	return "users"
}

// ToDTO преобразует в DTO
func (u *UserEntity) ToDTO() *dto.User {
	return &dto.User{
		Username: u.Username,
		Password: u.Password,
	}
}

// MessageEntity - модель сообщения для БД
type MessageEntity struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"userId"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User UserEntity `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName задает имя таблицы
func (MessageEntity) TableName() string {
	return "messages"
}

// ToDTO преобразует в DTO
func (m *MessageEntity) ToDTO() *dto.Message {
	username := ""
	if m.User.Username != "" {
		username = m.User.Username
	}

	return &dto.Message{
		Username:  username,
		Text:      m.Content,
		Timestamp: m.Timestamp,
	}
}

// SessionEntity - модель сессии для БД
type SessionEntity struct {
	ID        uint           `gorm:"primarykey" json:"-"`
	SessionID string         `gorm:"uniqueIndex;size:32;not null" json:"sessionId"`
	UserID    uint           `gorm:"not null;index" json:"userId"`
	ExpiresAt time.Time      `gorm:"not null" json:"expiresAt"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User UserEntity `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName задает имя таблицы
func (SessionEntity) TableName() string {
	return "sessions"
}

// ToDTO преобразует в DTO
func (s *SessionEntity) ToDTO() *dto.Session {
	username := ""
	if s.User.Username != "" {
		username = s.User.Username
	}

	return &dto.Session{
		Username: username,
	}
}

// IsExpired проверяет, истекла ли сессия
func (s *SessionEntity) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
