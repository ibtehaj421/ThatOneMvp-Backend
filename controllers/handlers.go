package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"health/anam/backend/database"
	"health/anam/backend/middleware"
	"health/anam/backend/models"
	"time"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// --- AUTHENTICATION ---

func RegisterUser(c *gin.Context) {
	var body struct {
		Username string      `json:"username" binding:"required"`
		Email    string      `json:"email" binding:"required,email"`
		Password string      `json:"password" binding:"required,min=6"`
		Role     models.Role `json:"role"` // Optional in request
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Security: Prevent users from making themselves system admins via the public API
	if body.Role == models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot register as admin"})
		return
	}
	
	// Default to patient if left empty or invalid
	if body.Role != models.RoleProvider {
		body.Role = models.RolePatient
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Username: body.Username, 
		Email:    body.Email, 
		Password: string(hash),
		Role:     body.Role,
	}

	if result := database.DB.Create(&user); result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User registered successfully", "role": user.Role})
}

func Login(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	database.DB.Where("email = ?", body.Email).First(&user)

	if user.ID == 0 || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Update: Pass the role into the token generator
	token, err := middleware.GenerateToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "role": user.Role})
}

func Logout(c *gin.Context) {
	// Note: True JWT logout requires a server-side blacklist (e.g., Redis). 
	// For standard stateless JWTs, you just tell the client side to delete the token.
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out. Please remove the token on the client side."})
}


// --- PROFILE / USER INFO ---

func GetUserInfo(c *gin.Context) {
	// Safely extract the user_id that our middleware attached to the context
	userID := c.GetUint("user_id")

	var user models.User
	
	// Query the database, but specifically exclude the password field for security
	result := database.DB.Select("id", "username", "email", "role", "created_at").First(&user, userID)
	
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": user,
	})
}
// --- ORGANIZATIONS ---

func RegisterOrganization(c *gin.Context) {
	var org models.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Assuming the user creating it is the owner
	org.OwnerEmail = c.GetString("email")

	if result := database.DB.Create(&org); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Organization created", "org": org})
}

// --- BOOKINGS / APPOINTMENTS ---

// CreateBooking handles appointment scheduling
func CreateBooking(c *gin.Context) {
	// 1. The Struct Fields MUST be Capitalized so Gin can access them.
	// We also add time_format to strictly handle the ISO8601 date from Postman.
	var body struct {
		PatientID      uint      `json:"patient_id" binding:"required"`
		ProviderID     uint      `json:"provider_id" binding:"required"`
		OrganizationID uint      `json:"organization_id" binding:"required"`
		StartTime      time.Time `json:"start_time" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	}

	// 2. Bind the incoming JSON payload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format: " + err.Error()})
		return
	}

	// 3. Map the payload to your actual Database Model
	booking := models.Appointment{
		PatientID:      body.PatientID,
		ProviderID:     body.ProviderID,
		OrganizationID: body.OrganizationID,
		StartTime:      body.StartTime,
		Status:         "pending", // Ensure default status is set
	}

	// 4. Save to the database
	if err := database.DB.Create(&booking).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create appointment: " + err.Error()})
		return
	}

	// 5. Return success with the new Appointment ID
	c.JSON(http.StatusOK, gin.H{
		"message":        "Appointment created successfully",
		"appointment_id": booking.ID,
		"booking":        booking,
	})
}

// --- DOCUMENT MANAGEMENT ---

func UploadDocument(c *gin.Context) {
	email := c.GetString("email") // Safely extracted from JWT

	file, err := c.FormFile("document")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No document uploaded"})
		return
	}

	// Create directory: ./storage/{user-email}/
	uploadDir := filepath.Join(".", "storage", email)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage directory"})
		return
	}

	// Save file
	filePath := filepath.Join(uploadDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("File %s uploaded successfully", file.Filename)})
}

func ListDocuments(c *gin.Context) {
	email := c.GetString("email")
	uploadDir := filepath.Join(".", "storage", email)

	files, err := os.ReadDir(uploadDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"documents": []string{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read documents"})
		return
	}

	var fileNames []string
	for _, f := range files {
		if !f.IsDir() {
			fileNames = append(fileNames, f.Name())
		}
	}

	c.JSON(http.StatusOK, gin.H{"documents": fileNames})
}