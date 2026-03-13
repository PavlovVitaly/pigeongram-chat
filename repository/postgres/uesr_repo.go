package postgres

import (
	"context"
	"errors"
	"time"

	"pigeongram/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create создает нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByUsername находит пользователя по имени
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // пользователь не найден
		}
		return nil, err
	}
	return &user, nil
}

// GetByID находит пользователя по ID
func (r *UserRepository) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateLastSeen обновляет время последнего визита
func (r *UserRepository) UpdateLastSeen(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_seen", time.Now()).Error
}

// Validate проверяет логин и пароль (временная версия, потом добавим хеширование)
func (r *UserRepository) Validate(ctx context.Context, username, password string) (bool, error) {
	user, err := r.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return false, err
	}
	// ВРЕМЕННО: прямое сравнение паролей (потом заменим на bcrypt)
	return user.Password == password, nil
}
