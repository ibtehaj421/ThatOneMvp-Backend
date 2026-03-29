package database

import (
	"log"
	"os"

	"health/anam/backend/models" // Update with your actual module path
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	// For local dev, you can hardcode this or use os.Getenv("DB_URL") if you use a package like godotenv
	// Example DSN: "host=localhost user=postgres password=yourpassword dbname=mvp_db port=5432 sslmode=disable"
	dsn := "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable TimeZone=Asia/Karachi"

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Prints SQL queries to console
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connection successfully opened")

	// Auto-migrate the schemas
	err = DB.AutoMigrate(&models.User{}, &models.Organization{}, &models.Appointment{})
	if err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	log.Println("Database schemas migrated successfully")
}