package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"`
	LastSeen  time.Time      `json:"lastSeen"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Messages []Message `json:"messages,omitempty" gorm:"foreignKey:UserID"`
}

// TableName задает имя таблицы
func (User) TableName() string {
	return "users"
}
