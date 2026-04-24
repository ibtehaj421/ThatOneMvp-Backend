package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// User represents the base user/patient in the system
type Role string

const (
	RolePatient  Role = "patient"
	RoleProvider Role = "provider" // For clinic owners/doctors
	RoleAdmin    Role = "admin"    // For system administrators
)

type User struct {
	ID        uint           `gorm:"primaryKey"`
	Username  string         `gorm:"uniqueIndex;not null"`
	Email     string         `gorm:"uniqueIndex;not null"`
	Password  string         `gorm:"not null"`
	Role      Role           `gorm:"type:varchar(20);default:'patient'"` // Added Role
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Organization represents a clinic, hospital, or general org
type Organization struct {
	ID                   uint           `gorm:"primaryKey"`
	Name                 string         `gorm:"not null"`
	Latitude             float64        // Map location
	Longitude            float64        // Map location
	CustomerSupportEmail string
	OwnerEmail           string         `gorm:"not null"`
	AdminEmails          pq.StringArray `gorm:"type:text[]"` // Postgres specific array type
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

// AppointmentStatus defines the state of a booking
type AppointmentStatus string

const (
	StatusPending   AppointmentStatus = "pending"
	StatusConfirmed AppointmentStatus = "confirmed"
	StatusCancelled AppointmentStatus = "cancelled"
	StatusCompleted AppointmentStatus = "completed"
)

// Appointment tracks patient bookings at specific organizations
type Appointment struct {
	ID             uint              `gorm:"primaryKey"`
	PatientID      uint              `gorm:"not null;index"` // Foreign key to User
	ProviderID     uint              `gorm:"not null;index"`
	OrganizationID uint              `gorm:"not null;index"` // Foreign key to Organization
	StartTime      time.Time         `gorm:"not null"`
	EndTime        time.Time
	Status         AppointmentStatus `gorm:"type:varchar(20);default:'pending'"`
	Notes          string            `gorm:"type:text"` // For patient symptoms or doctor notes
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Relationships
	Patient      User         `gorm:"foreignKey:PatientID"`
	Provider     User         `gorm:"foreignKey:ProviderID"`
	Organization Organization `gorm:"foreignKey:OrganizationID"`
}

// InterviewSession tracks the stateless conversational history for the AI
type InterviewSession struct {
	// Composite Primary Key: PatientID + SessionSeq
	PatientID  uint `gorm:"primaryKey;autoIncrement:false"`
	SessionSeq uint `gorm:"primaryKey;autoIncrement:false"`

	Status          string `gorm:"type:varchar(20);default:'in_progress'"` // 'in_progress', 'completed'
	DialogueHistory string `gorm:"type:jsonb;default:'[]'"`                // Stores the array of messages
	ExtractedSlots  string `gorm:"type:jsonb;default:'{}'"`                // Stores the current MediTOD state
	
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relationships
	Patient User `gorm:"foreignKey:PatientID"`
}


type SymptomSlot struct {
	Value       string `json:"value"`
	Onset       string `json:"onset"`
	Duration    string `json:"duration"`
	Severity    string `json:"severity"`
	Location    string `json:"location"`
	Progression string `json:"progression"`
	Frequency   string `json:"frequency"`
}

type CMASState struct {
	ChiefComplaint        string                 `json:"chief_complaint"`
	PositiveSymptoms      []SymptomSlot          `json:"positive_symptoms"`
	NegativeSymptoms      []SymptomSlot          `json:"negative_symptoms"`
	PatientMedicalHistory []string               `json:"patient_medical_history"`
	FamilyMedicalHistory  []string               `json:"family_medical_history"`
	Medications           []string               `json:"medications"`
	Habits                map[string]interface{} `json:"habits"`
	BasicInformation      map[string]interface{} `json:"basic_information"`
}