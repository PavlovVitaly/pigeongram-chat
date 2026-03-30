package postgres

import (
	"context"
	"errors"
	"time"

	"pigeongram/app/internal/models"
	dto "pigeongram/app/pkg/models"

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
	entity := &models.UserEntity{
		Username: user.Username,
		Password: user.Password,
	}
	return r.db.WithContext(ctx).Create(entity).Error
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

// Validate проверяет логин и пароль (временная версия, потом добавим хеширование)
func (r *UserRepository) Validate(ctx context.Context, username, password string) (bool, error) {
	user, err := r.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return false, err
	}
	return user.Password == password, nil
}

// Count возвращает общее количество пользователей
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.UserEntity{}).Count(&count).Error
	return count, err
}
