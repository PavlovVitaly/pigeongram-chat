package postgres

import (
	"context"

	"pigeongram/models"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create сохраняет новое сообщение
func (r *MessageRepository) Create(ctx context.Context, message *models.Message) error {
	return r.db.WithContext(ctx).Create(message).Error
}

// GetRecent возвращает последние N сообщений с информацией о пользователях
func (r *MessageRepository) GetRecent(ctx context.Context, limit int) ([]models.Message, error) {
	var messages []models.Message

	err := r.db.WithContext(ctx).
		Preload("User"). // Загружаем связанного пользователя
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
func (r *MessageRepository) GetHistory(ctx context.Context, offset, limit int) ([]models.Message, error) {
	var messages []models.Message

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
