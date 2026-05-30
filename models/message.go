package models

import (
	"time"

	"gorm.io/gorm"
)

type Message struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"userId"`
	Content   string         `gorm:"type:text;not null" json:"text"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Edited    bool           `gorm:"default:false" json:"edited"`
	EditedAt  *time.Time     `json:"editedAt,omitempty"`
	User      User           `gorm:"foreignKey:UserID" json:"user"`
}

// TableName задает имя таблицы
func (Message) TableName() string {
	return "messages"
}
