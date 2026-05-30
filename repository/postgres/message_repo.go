package postgres

import (
	"context"
	"fmt"
	"time"

	"pigeongram-chat/internal/models"
	dto "pigeongram-chat/pkg/models" // DTO

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create сохраняет новое сообщение
func (r *MessageRepository) Create(ctx context.Context, msg *dto.Message) (uint, error) {
	var user models.UserEntity
	if err := r.db.WithContext(ctx).Where("username = ?", msg.Username).First(&user).Error; err != nil {
		return 0, err
	}
	entity := &models.MessageEntity{
		UserID:    user.ID,
		Content:   msg.Text,
		Timestamp: msg.Timestamp,
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return 0, err
	}
	return entity.ID, nil
}

// UpdateMessage обновляет текст и выставляет edited=true
func (r *MessageRepository) UpdateMessage(ctx context.Context, msgID uint, newText string, username string) error {
	var msg models.MessageEntity
	if err := r.db.WithContext(ctx).Preload("User").First(&msg, msgID).Error; err != nil {
		return err
	}
	if msg.User.Username != username {
		return fmt.Errorf("редактировать можно только свои сообщения")
	}
	now := time.Now()
	return r.db.WithContext(ctx).Model(&msg).Updates(map[string]interface{}{
		"content":    newText,
		"edited":     true,
		"updated_at": now,
		"edited_at":  now,
	}).Error
}

// GetRecent возвращает последние N сообщений с информацией о пользователях
func (r *MessageRepository) GetRecent(ctx context.Context, limit int) ([]models.MessageEntity, error) {
	var messages []models.MessageEntity

	err := r.db.WithContext(ctx).
		Preload("User").
		Order("timestamp desc").
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, err
	}

	// Переворачиваем, чтобы шли от старых к новым
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetHistory возвращает сообщения с пагинацией
func (r *MessageRepository) GetHistory(ctx context.Context, offset, limit int) ([]models.MessageEntity, error) {
	var messages []models.MessageEntity

	err := r.db.WithContext(ctx).
		Preload("User").
		Order("timestamp desc").
		Offset(offset).
		Limit(limit).
		Find(&messages).Error

	return messages, err
}

// Delete старые сообщения (для очистки)
func (r *MessageRepository) DeleteOlderThan(ctx context.Context, days int) error {
	// TODO: реализовать удаление старых сообщений
	return nil
}

// Count возвращает общее количество сообщений
func (r *MessageRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.MessageEntity{}).Count(&count).Error
	return count, err
}
