package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"pigeongram-chat/internal/models"
	"pigeongram-chat/pkg/crypto"
	dto "pigeongram-chat/pkg/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create создает нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *dto.User) error {
	// Хешируем пароль
	hashedPassword, err := crypto.HashPassword(user.Password)
	if err != nil {
		return fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	entity := &models.UserEntity{
		Username: user.Username,
		Password: hashedPassword,
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *UserRepository) Validate(ctx context.Context, username, password string) (bool, error) {
	log.Printf("🔍 [VALIDATE] Checking user: %s", username)

	user, err := r.GetByUsername(ctx, username)
	if err != nil {
		log.Printf("❌ [VALIDATE] DB error: %v", err)
		return false, err
	}
	if user == nil {
		log.Printf("❌ [VALIDATE] User not found: %s", username)
		return false, nil
	}

	log.Printf("🔍 [VALIDATE] User found: %s", username)
	log.Printf("🔍 [VALIDATE] Stored hash length: %d", len(user.Password))
	log.Printf("🔍 [VALIDATE] Stored hash: %s", user.Password)
	log.Printf("🔍 [VALIDATE] Input password: %s", password)

	// Сравниваем пароль с хешем
	result := crypto.CheckPasswordHash(password, user.Password)
	log.Printf("🔍 [VALIDATE] Password match result: %v", result)

	if !result {
		log.Printf("❌ [VALIDATE] Password mismatch for user: %s", username)
	}

	return result, nil
}

// GetByUsername находит пользователя по имени
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.UserEntity, error) {
	var user models.UserEntity
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByID находит пользователя по ID
func (r *UserRepository) GetByID(ctx context.Context, id uint) (*models.UserEntity, error) {
	var user models.UserEntity
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateLastSeen обновляет время последнего визита
func (r *UserRepository) UpdateLastSeen(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Model(&models.UserEntity{}).
		Where("id = ?", userID).
		Update("last_seen", time.Now()).Error
}

// Count возвращает общее количество пользователей
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.UserEntity{}).Count(&count).Error
	return count, err
}
