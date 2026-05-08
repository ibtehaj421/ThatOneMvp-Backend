package database

import (
	"log"
	"os"
	"fmt"
	"health/anam/backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/joho/godotenv"
)

var DB *gorm.DB

func ConnectDB() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system default environment variables")
	}
	host := getEnv("DB_HOST", "db")
	user := getEnv("DB_USER", "anam_user")
	password := getEnv("DB_PASSWORD", "anam_password")
	dbName := getEnv("DB_NAME", "anam_db")
	port := getEnv("DB_PORT", "5435")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Karachi", 
		host, user, password, dbName, port)
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connection successfully opened")

	// Added AppointmentMessage and Notification
	err = DB.AutoMigrate(
		&models.User{}, 
		&models.Organization{}, 
		&models.Appointment{},
		&models.InterviewSession{},
		&models.AppointmentMessage{},
		&models.Notification{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	log.Println("Database schemas migrated successfully")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}