package db

import (
	"fmt"
	"log/slog"
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
		slog.Error("db.connect_failed", "event", "db.connect_failed", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("db.connected", "event", "db.connected")
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Work{},
		&models.PublishingOrder{},
		&models.OrderWork{},
	)
	if err != nil {
		slog.Error("db.migrate_failed", "event", "db.migrate_failed", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("db.migrated", "event", "db.migrated")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
