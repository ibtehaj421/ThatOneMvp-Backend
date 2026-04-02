package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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



// --- CLINICAL EXPORT HELPERS ---

func formatSymptoms(symptoms []models.SymptomSlot) string {
	if len(symptoms) == 0 {
		return "  None reported.\n"
	}
	var sb strings.Builder
	for _, sym := range symptoms {
		val := "Unknown symptom"
		if sym.Value != "" {
			val = strings.ToUpper(sym.Value)
		}
		sb.WriteString(fmt.Sprintf("  • %s\n", val))

		appendAttr(&sb, "Onset", sym.Onset)
		appendAttr(&sb, "Duration", sym.Duration)
		appendAttr(&sb, "Severity", sym.Severity)
		appendAttr(&sb, "Location", sym.Location)
		appendAttr(&sb, "Progression", sym.Progression)
		appendAttr(&sb, "Frequency", sym.Frequency)
	}
	return sb.String()
}

func appendAttr(sb *strings.Builder, key, value string) {
	if value != "" {
		sb.WriteString(fmt.Sprintf("      └ %s: %s\n", key, value))
	}
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "  None reported.\n"
	}
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("  • %s\n", item))
	}
	return sb.String()
}


// --- EXPORT ENDPOINT ---

// ExportClinicalHistory generates a text file of the patient's intake history
func ExportClinicalHistory(c *gin.Context) {
	patientID := c.GetUint("user_id")
	sessionSeqStr := c.Param("session_seq")

	// 1. Fetch the session from the database
	var session models.InterviewSession
	result := database.DB.Where("patient_id = ? AND session_seq = ?", patientID, sessionSeqStr).First(&session)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// 2. Unmarshal the Extracted Slots
	var state models.CMASState // Note: Referencing the struct from the models package
	if err := json.Unmarshal([]byte(session.ExtractedSlots), &state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse clinical data"})
		return
	}

	// 3. Extract ICE (Ideas, Concerns, Expectations)
	ice := "Not reported."
	if val, ok := state.BasicInformation["concerns_and_expectations"].(string); ok && val != "" {
		ice = val
	}

	// 4. Extract Habits
	var habitsStr strings.Builder
	if len(state.Habits) > 0 {
		for k, v := range state.Habits {
			habitsStr.WriteString(fmt.Sprintf("  • %s: %v\n", strings.Title(k), v))
		}
	} else {
		habitsStr.WriteString("  Not reported.\n")
	}

	// 5. Build the Output String
	pc := state.ChiefComplaint
	if pc == "" {
		pc = "Not specified."
	}

	output := fmt.Sprintf(`╔══════════════════════════════════════════════════════════╗
║             CLINICAL INTAKE HISTORY                      ║
╚══════════════════════════════════════════════════════════╝

PRESENTING COMPLAINT (PC):
  %s

HISTORY OF PRESENTING COMPLAINT (HPC):
%s
IDEAS, CONCERNS & EXPECTATIONS (ICE):
  • %s

SYSTEMS REVIEW:
%s
PAST MEDICAL HISTORY (PMH):
%s
DRUG HISTORY & ALLERGIES (DH):
%s
FAMILY HISTORY (FH):
%s
SOCIAL HISTORY (SH):
%s
════════════════════════════════════════════════════════════
  Generated by ANAM-AI Intake Agent
════════════════════════════════════════════════════════════
`, 
		pc, 
		formatSymptoms(state.PositiveSymptoms),
		ice,
		formatSymptoms(state.NegativeSymptoms),
		formatList(state.PatientMedicalHistory),
		formatList(state.Medications),
		formatList(state.FamilyMedicalHistory),
		habitsStr.String(),
	)

	// 6. Send as downloadable text file
	fileName := fmt.Sprintf("Intake_History_Session_%s.txt", sessionSeqStr)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "text/plain")
	
	c.String(http.StatusOK, output)
}


// --- SESSION RETRIEVAL ENDPOINTS ---

// GetAllSessions retrieves a lightweight list of all interview sessions for the logged-in patient
func GetAllSessions(c *gin.Context) {
	patientID := c.GetUint("user_id")

	// We create a lightweight struct so we don't pull down the heavy JSONB
	// columns (DialogueHistory, ExtractedSlots) just to display a list.
	type SessionSummary struct {
		SessionSeq uint      `json:"session_seq"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
		UpdatedAt  time.Time `json:"updated_at"`
	}

	var summaries []SessionSummary

	// Query the database, ordering by the most recent sessions first
	result := database.DB.Model(&models.InterviewSession{}).
		Where("patient_id = ?", patientID).
		Order("session_seq desc").
		Find(&summaries)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sessions"})
		return
	}

	// Return an empty array instead of null if there are no sessions
	if summaries == nil {
		summaries = []SessionSummary{}
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": summaries,
	})
}

// GetSessionHistory retrieves the chat messages (DialogueHistory) for a specific session
func GetSessionHistory(c *gin.Context) {
	patientID := c.GetUint("user_id")
	sessionSeqStr := c.Param("session_seq")

	var session models.InterviewSession

	// We only need to select the DialogueHistory column here
	result := database.DB.Select("dialogue_history").
		Where("patient_id = ? AND session_seq = ?", patientID, sessionSeqStr).
		First(&session)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Since DialogueHistory is stored as a JSONB string in Postgres, 
	// we use json.RawMessage to pass it directly to Gin without unmarshaling/marshaling overhead.
	var rawHistory json.RawMessage
	if session.DialogueHistory != "" {
		rawHistory = json.RawMessage(session.DialogueHistory)
	} else {
		rawHistory = json.RawMessage("[]") // Default empty array
	}

	c.JSON(http.StatusOK, gin.H{
		"session_seq": sessionSeqStr,
		"history":     rawHistory,
	})
}