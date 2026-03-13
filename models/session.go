package models

import (
	"time"

	"gorm.io/gorm"
)

type Session struct {
	ID        uint           `gorm:"primarykey" json:"-"`
	SessionID string         `gorm:"uniqueIndex;size:32;not null" json:"sessionId"`
	UserID    uint           `gorm:"not null;index" json:"userId"`
	ExpiresAt time.Time      `gorm:"not null" json:"expiresAt"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName задает имя таблицы
func (Session) TableName() string {
	return "sessions"
}

// IsExpired проверяет, истекла ли сессия
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
