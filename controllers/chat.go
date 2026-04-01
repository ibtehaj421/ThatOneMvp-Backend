package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"health/anam/backend/database"
	"health/anam/backend/models"

	"github.com/gin-gonic/gin"
)

// --- Structs for Requests/Responses ---

type StartSessionResponse struct {
	SessionSeq uint   `json:"session_seq"`
	Status     string `json:"status"`
}

type ChatMessageRequest struct {
	SessionSeq uint   `json:"session_seq" binding:"required"`
	Message    string `json:"message" binding:"required"`
}

// Payload sent to the Python AI Worker
type PythonAgentPayload struct {
	PatientID  uint   `json:"patient_id"`
	SessionSeq uint   `json:"session_seq"`
	Message    string `json:"message"`
}

// Expected response from Python AI Worker
type PythonAgentResponse struct {
	Reply      string `json:"reply"`
	IsComplete bool   `json:"is_complete"`
}

// --- Endpoints ---

// StartSession generates a scoped Session ID for the user and creates a new DB record
func StartSession(c *gin.Context) {
	patientID := c.GetUint("user_id")

	// 1. Find the current highest SessionSeq for this specific patient
	var maxSeq struct {
		MaxSeq uint
	}
	
	// Query the max sequence; if none exists, it defaults to 0
	database.DB.Model(&models.InterviewSession{}).
		Select("COALESCE(MAX(session_seq), 0) as max_seq").
		Where("patient_id = ?", patientID).
		Scan(&maxSeq)

	nextSeq := maxSeq.MaxSeq + 1

	// 2. Initialize the new session in the database
	newSession := models.InterviewSession{
		PatientID:  patientID,
		SessionSeq: nextSeq,
		Status:     "in_progress",
		// JSONB fields default to '[]' and '{}' via GORM tags
	}

	if result := database.DB.Create(&newSession); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize chat session"})
		return
	}

	c.JSON(http.StatusOK, StartSessionResponse{
		SessionSeq: nextSeq,
		Status:     "Session initialized successfully",
	})
}

// SendMessage acts as the proxy to the stateless Python module
func SendMessage(c *gin.Context) {
	var req ChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	patientID := c.GetUint("user_id")

	// 1. Verify the session exists and belongs to the user
	var session models.InterviewSession
	result := database.DB.Where("patient_id = ? AND session_seq = ?", patientID, req.SessionSeq).First(&session)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This interview session has already been completed"})
		return
	}

	// 2. Prepare payload for Python
	agentPayload := PythonAgentPayload{
		PatientID:  patientID,
		SessionSeq: req.SessionSeq, // Python will use the tuple (PatientID, SessionSeq) to query pgvector/state
		Message:    req.Message,
	}

	payloadBytes, _ := json.Marshal(agentPayload)

	// 3. Forward to Python
	pythonURL := os.Getenv("PYTHON_AGENT_URL")
	if pythonURL == "" {
		pythonURL = "http://localhost:8000/chat" 
	}

	resp, err := http.Post(pythonURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI worker is currently offline"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("AI worker failed with status: %d", resp.StatusCode)})
		return
	}

	// 4. Decode Python Response
	var agentResp PythonAgentResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &agentResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	// Note: We do NOT save the history here. Python is handling the JSONB writes directly 
	// to `PatientInterviewSessions` to remain the single source of truth for the MediTOD state.

	// 5. Send final reply to React
	c.JSON(http.StatusOK, gin.H{
		"session_seq": req.SessionSeq,
		"reply":       agentResp.Reply,
		"is_complete": agentResp.IsComplete,
	})
}