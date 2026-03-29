package main

import (
	"log"

	"health/anam/backend/database" // Update with your actual module path
)

func main() {
	log.Println("Starting MVP Backend...")

	// Initialize Database Connection and Auto-Migrate Models
	database.ConnectDB()

	log.Println("Backend is up and running. Ready to accept connections.")
	
	// Server logic (e.g., Gin, Fiber, or net/http) will go here
}