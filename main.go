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
	middleware.DB = database.DB
	r := gin.Default()

	// Public Routes
	r.POST("/register", controllers.RegisterUser)
	r.POST("/login", controllers.Login)
	// --- NEW: WEBSOCKET ROUTE ---
		// Note: It's outside the JWT middleware because browsers don't send auth headers with WebSockets.
		// Security is handled via token/ID validation inside the handler.
		r.GET("/ws/chat", controllers.HandleWebSocket)
	// Protected Routes (Require valid JWT)
	protected := r.Group("/")
	protected.Use(middleware.RequireAuth())
	{
		protected.POST("/logout", controllers.Logout)

		// ---------------------------------------------------------
		// PATIENT & PROVIDER SHARED ROUTES
		// ---------------------------------------------------------
		protected.POST("/bookings", controllers.CreateBooking)
		protected.POST("/documents/upload", controllers.UploadDocument)
		protected.GET("/appointments", controllers.GetUserAppointments)
		protected.GET("/documents", controllers.ListDocuments)
		protected.GET("/profile", controllers.GetUserInfo)
		protected.PUT("/profile", controllers.UpdateProfile)
		
		// Note: The /chat endpoints here assume you have your StartSession/SendMessage in a separate file,
		// Ensure they are available, otherwise comment these out if missing in the context
		protected.POST("/chat/start", controllers.StartSession)
		protected.POST("/chat/message", controllers.SendMessage)
		protected.GET("/chat/export/:session_seq", controllers.ExportClinicalHistory)
		protected.GET("/chat/sessions", controllers.GetAllSessions)
		protected.GET("/chat/sessions/:session_seq/history", controllers.GetSessionHistory)
		
		protected.GET("/organizations", controllers.GetNearbyOrganizations)
		protected.GET("/providers", controllers.GetAllProviders)

		// --- NEW: APPOINTMENT CHAT ROUTES ---
		protected.POST("/appointments/:appointment_id/chat", controllers.SendAppointmentMessage)
		protected.GET("/appointments/:appointment_id/chat", controllers.GetAppointmentMessages)

		// --- NEW: NOTIFICATIONS ROUTES ---
		protected.GET("/notifications", controllers.GetMyNotifications)
		protected.PUT("/notifications/read", controllers.MarkNotificationsRead)
		
		
		// ---------------------------------------------------------
		// PROVIDER / ADMIN ONLY ROUTES
		// ---------------------------------------------------------
		providerOnly := protected.Group("/")
		providerOnly.Use(middleware.RequireRole(models.RoleProvider, models.RoleAdmin))
		{
			providerOnly.POST("/organizations", controllers.RegisterOrganization)
			providerOnly.GET("/organizations/:org_id/appointments", controllers.GetOrganizationAppointments)
			providerOnly.GET("/appointments/:appointment_id/context", controllers.GetPatientClinicalContext)
			providerOnly.GET("/my-organizations", controllers.GetProviderOrganizations)
			providerOnly.PUT("/appointments/:appointment_id/notes", controllers.UpdateDoctorNotes)
			
			// --- NEW: SCHEDULE MANAGEMENT ROUTES ---
			// Allows the doctor to mark someone as missed on their dashboard (like an excel sheet)
			providerOnly.PUT("/appointments/:appointment_id/missed", controllers.MarkAppointmentMissed)
		}
	}

	log.Println("Backend is up and running on :8080. Ready to accept connections.")
	r.Run(":8080")
}