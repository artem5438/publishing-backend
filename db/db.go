package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"publishing-backend/models"
)

var DB *gorm.DB

func Connect() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Novosibirsk",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "admin"),
		getEnv("DB_PASSWORD", "secret"),
		getEnv("DB_NAME", "lab2_db"),
		getEnv("DB_PORT", "5432"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	log.Println("✅ PostgreSQL подключён")
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Work{},
		&models.PublishingOrder{},
		&models.OrderWork{},
	)
	if err != nil {
		log.Fatal("❌ Ошибка миграции:", err)
	}
	log.Println("✅ Таблицы созданы/обновлены")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
