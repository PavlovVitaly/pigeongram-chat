package main

import (
	"fmt"
	"log"
	"os"
	"pigeongram/app/internal/models"
	"pigeongram/app/pkg/crypto"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "host=postgres port=5432 user=pigeongram password=pigeongram123 dbname=pigeongram sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Ошибка подключения:", err)
	}

	var users []models.UserEntity
	db.Find(&users)

	updated := 0
	for _, user := range users {
		// Проверяем, не хеширован ли уже пароль
		if len(user.Password) == 60 && (user.Password[:4] == "$2a$" || user.Password[:4] == "$2b$") {
			fmt.Printf("⏭️ Пользователь %s уже имеет хеш\n", user.Username)
			continue
		}

		// Хешируем существующий пароль
		hashed, err := crypto.HashPassword(user.Password)
		if err != nil {
			log.Printf("❌ Ошибка хеширования для %s: %v", user.Username, err)
			continue
		}

		// Обновляем в БД
		db.Model(&user).Update("password", hashed)
		fmt.Printf("✅ Пользователь %s обновлен\n", user.Username)
		updated++
	}

	fmt.Printf("\n📊 Итого обновлено: %d пользователей\n", updated)
}
