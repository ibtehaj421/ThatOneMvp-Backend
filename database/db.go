package database

import (
	"log"
	

	"health/anam/backend/models" // Update with your actual module path
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	
	dsn := "host=127.0.0.1 user=postgres password=mysecretpassword dbname=postgres port=5435 sslmode=disable TimeZone=Asia/Karachi"

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