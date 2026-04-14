package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword хеширует пароль с помощью bcrypt
func HashPassword(password string) (string, error) {
	// bcrypt.DefaultCost = 10
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash проверяет пароль на соответствие хешу
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
