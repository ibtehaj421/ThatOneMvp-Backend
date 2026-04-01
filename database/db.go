package database

import (
	"log"
	"os"
	"fmt"
	"health/anam/backend/models" // Update with your actual module path
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/joho/godotenv"
)

var DB *gorm.DB

func ConnectDB() {
	
	//dsn := "host=127.0.0.1 user=postgres password=mysecretpassword dbname=postgres port=5435 sslmode=disable TimeZone=Asia/Karachi"
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system default environment variables")
	}
	host := getEnv("DB_HOST", "localhost")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "postgres")
	port := getEnv("DB_PORT", "5432")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Karachi", 
		host, user, password, dbName, port)
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Prints SQL queries to console
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connection successfully opened")

	// Auto-migrate the schemas
	err = DB.AutoMigrate(
		&models.User{}, 
		&models.Organization{}, 
		&models.Appointment{},
		&models.InterviewSession{},
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