package main

import (
	"log"
	"health/anam/backend/controllers"
	"health/anam/backend/database"
	"health/anam/backend/middleware"
	"health/anam/backend/models"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting MVP Backend...")

	database.ConnectDB()
	r := gin.Default()

	// Public Routes
	r.POST("/register", controllers.RegisterUser)
	r.POST("/login", controllers.Login)

	// Protected Routes (Require valid JWT)
	protected := r.Group("/")
	protected.Use(middleware.RequireAuth())
	{
		protected.POST("/logout", controllers.Logout)

		// ---------------------------------------------------------
		// PATIENT & PROVIDER SHARED ROUTES
		// ---------------------------------------------------------
		// Both roles can upload/view documents and create bookings
		protected.POST("/bookings", controllers.CreateBooking)
		protected.POST("/documents/upload", controllers.UploadDocument)
		protected.GET("/documents", controllers.ListDocuments)
		protected.GET("/profile", controllers.GetUserInfo)

		// ---------------------------------------------------------
		// PROVIDER / ADMIN ONLY ROUTES
		// ---------------------------------------------------------
		providerOnly := protected.Group("/")
		providerOnly.Use(middleware.RequireRole(models.RoleProvider, models.RoleAdmin))
		{
			// Only providers and admins can register a new hospital/clinic
			providerOnly.POST("/organizations", controllers.RegisterOrganization)
		}
	}

	log.Println("Backend is up and running on :8080. Ready to accept connections.")
	r.Run(":8080")
}