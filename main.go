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
		protected.GET("/appointments", controllers.GetUserAppointments)
		protected.GET("/documents", controllers.ListDocuments)
		protected.GET("/profile", controllers.GetUserInfo)
		protected.POST("/chat/start", controllers.StartSession)
		protected.POST("/chat/message", controllers.SendMessage)
		protected.GET("/chat/export/:session_seq", controllers.ExportClinicalHistory)
		protected.GET("/chat/sessions", controllers.GetAllSessions)
		protected.GET("/chat/sessions/:session_seq/history", controllers.GetSessionHistory)
		protected.GET("/organizations", controllers.GetNearbyOrganizations)
		protected.GET("/providers", controllers.GetAllProviders)
		// ---------------------------------------------------------
		// PROVIDER / ADMIN ONLY ROUTES
		// ---------------------------------------------------------
		providerOnly := protected.Group("/")
		// Restrict these routes to ONLY users with the Provider or Admin role
		providerOnly.Use(middleware.RequireRole(models.RoleProvider, models.RoleAdmin))
		{
			// Organization management
			providerOnly.POST("/organizations", controllers.RegisterOrganization)

			providerOnly.GET("/organizations/:org_id/appointments", controllers.GetOrganizationAppointments)
			// Fetch the full clinical context (AI history + demographics) for a specific appointment
			providerOnly.GET("/appointments/:appointment_id/context", controllers.GetPatientClinicalContext)
			providerOnly.GET("/my-organizations", controllers.GetProviderOrganizations)
			// Save the doctor's final clinical notes
			providerOnly.PUT("/appointments/:appointment_id/notes", controllers.UpdateDoctorNotes)
		}
	}

	log.Println("Backend is up and running on :8080. Ready to accept connections.")
	r.Run(":8080")
}