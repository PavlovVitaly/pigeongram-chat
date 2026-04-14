package postgres

import (
	"context"
	"errors"
	"time"

	"pigeongram/internal/models"

	"gorm.io/gorm"
)

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create создает новую сессию
func (r *SessionRepository) Create(ctx context.Context, session *models.SessionEntity) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID находит сессию по ID
func (r *SessionRepository) GetByID(ctx context.Context, sessionID string) (*models.SessionEntity, error) {
	var session models.SessionEntity
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("session_id = ?", sessionID).
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// Delete удаляет сессию
func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Delete(&models.SessionEntity{}).Error
}

// Cleanup удаляет истекшие сессии
func (r *SessionRepository) Cleanup(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.SessionEntity{}).Error
}

// CreateUserSession создает сессию для пользователя
func (r *SessionRepository) CreateUserSession(ctx context.Context, userID uint, sessionID string) error {
	session := &models.SessionEntity{
		SessionID: sessionID,
		UserID:    userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	return r.Create(ctx, session)
}
