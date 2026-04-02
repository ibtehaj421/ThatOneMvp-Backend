package controllers

import (
	"encoding/json"
	"net/http"

	"health/anam/backend/database"
	"health/anam/backend/models"

	"github.com/gin-gonic/gin"
)

// --- PROVIDER ENDPOINTS ---

// GetPatientClinicalContext securely fetches an appointment and the patient's AI history.
// It fails if the requesting doctor is not assigned to the appointment.
func GetPatientClinicalContext(c *gin.Context) {
	providerID := c.GetUint("user_id") // From your JWT middleware
	appointmentID := c.Param("appointment_id")

	// 1. Authorization Check: Does this doctor have THIS appointment?
	var appointment models.Appointment
	result := database.DB.Preload("Patient").Where("id = ? AND provider_id = ?", appointmentID, providerID).First(&appointment)
	if result.Error != nil {
		// Return a generic 404/403 to prevent data leakage
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized access or appointment not found"})
		return
	}

	// 2. Fetch the Patient's AI Intake History
	// We fetch the most recently completed AI session for this patient.
	var latestSession models.InterviewSession
	database.DB.Where("patient_id = ? AND status = ?", appointment.PatientID, "completed").
		Order("created_at desc").
		First(&latestSession)

	// Unmarshal the extracted slots so the frontend gets clean JSON
	var parsedHistory models.CMASState
	if latestSession.ExtractedSlots != "" {
		json.Unmarshal([]byte(latestSession.ExtractedSlots), &parsedHistory)
	}

	// 3. Return the payload to the Doctor's Portal
	c.JSON(http.StatusOK, gin.H{
		"appointment_details": gin.H{
			"appointment_id": appointment.ID,
			"status":         appointment.Status,
			"start_time":     appointment.StartTime,
			"doctor_notes":   appointment.Notes, // Previous notes written by the doctor
		},
		"patient_demographics": gin.H{
			"patient_id": appointment.Patient.ID,
			"username":   appointment.Patient.Username,
			"email":      appointment.Patient.Email,
		},
		"ai_intake_history": parsedHistory, // The structured CMAS JSON from Python
	})
}

// UpdateDoctorNotes allows the treating doctor to save their final clinical notes
// back to the appointment record.
func UpdateDoctorNotes(c *gin.Context) {
	providerID := c.GetUint("user_id")
	appointmentID := c.Param("appointment_id")

	var body struct {
		Notes  string `json:"notes" binding:"required"`
		Status string `json:"status"` // e.g., to mark the appointment as "completed"
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Authorization Check
	var appointment models.Appointment
	if err := database.DB.Where("id = ? AND provider_id = ?", appointmentID, providerID).First(&appointment).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized access"})
		return
	}

	// 2. Update the fields
	appointment.Notes = body.Notes
	if body.Status != "" {
		// Cast the string to your AppointmentStatus type
		appointment.Status = models.AppointmentStatus(body.Status) 
	}

	if err := database.DB.Save(&appointment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save notes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Clinical notes updated successfully",
		"notes":   appointment.Notes,
	})
}