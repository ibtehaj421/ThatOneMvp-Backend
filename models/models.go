package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Role string

const (
	RolePatient  Role = "patient"
	RoleProvider Role = "provider"
	RoleAdmin    Role = "admin"
)

type User struct {
	ID                   uint           `gorm:"primaryKey"`
	Username             string         `gorm:"uniqueIndex;not null"`
	Email                string         `gorm:"uniqueIndex;not null"`
	Password             string         `gorm:"not null"`
	Role                 Role           `gorm:"type:varchar(20);default:'patient'"`
	// --- NEW FIELDS FOR PATIENT INFO ---
	FullName             string         `gorm:"type:varchar(255)"`
	IdentificationNumber string         `gorm:"type:varchar(100)"` // e.g., SSN, National ID
	Location             string         `gorm:"type:text"`         // Address or coordinates
	ProfileImageURL      string         `gorm:"type:text"`         // S3 link or local path
	// -----------------------------------
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

type Organization struct {
	ID                   uint           `gorm:"primaryKey"`
	Name                 string         `gorm:"not null"`
	Latitude             float64        
	Longitude            float64        
	CustomerSupportEmail string
	OwnerEmail           string         `gorm:"not null"`
	AdminEmails          pq.StringArray `gorm:"type:text[]"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

type AppointmentStatus string

const (
	StatusPending   AppointmentStatus = "pending"
	StatusConfirmed AppointmentStatus = "confirmed"
	StatusCancelled AppointmentStatus = "cancelled"
	StatusCompleted AppointmentStatus = "completed"
	StatusMissed    AppointmentStatus = "missed" // --- NEW STATUS ---
)

type Appointment struct {
	ID             uint              `gorm:"primaryKey"`
	PatientID      uint              `gorm:"not null;index"`
	ProviderID     uint              `gorm:"not null;index"`
	OrganizationID uint              `gorm:"not null;index"`
	StartTime      time.Time         `gorm:"not null"`
	EndTime        time.Time
	Status         AppointmentStatus `gorm:"type:varchar(20);default:'pending'"`
	Notes          string            `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time

	Patient      User         `gorm:"foreignKey:PatientID"`
	Provider     User         `gorm:"foreignKey:ProviderID"`
	Organization Organization `gorm:"foreignKey:OrganizationID"`
}

// --- NEW MODEL: Chat System for Appointments ---
type AppointmentMessage struct {
	ID            uint      `gorm:"primaryKey"`
	AppointmentID uint      `gorm:"not null;index"`
	SenderID      uint      `gorm:"not null"` // Can be Patient or Provider
	Message       string    `gorm:"type:text;not null"`
	CreatedAt     time.Time
	
	Appointment Appointment `gorm:"foreignKey:AppointmentID"`
	Sender      User        `gorm:"foreignKey:SenderID"`
}

// --- NEW MODEL: Notifications System ---
type Notification struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"` // Person receiving the notification
	Message   string    `gorm:"type:text;not null"`
	IsRead    bool      `gorm:"default:false"`
	CreatedAt time.Time

	User User `gorm:"foreignKey:UserID"`
}

type InterviewSession struct {
	PatientID  uint `gorm:"primaryKey;autoIncrement:false"`
	SessionSeq uint `gorm:"primaryKey;autoIncrement:false"`
	Status          string `gorm:"type:varchar(20);default:'in_progress'"`
	DialogueHistory string `gorm:"type:jsonb;default:'[]'"`
	ExtractedSlots  string `gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time
	UpdatedAt time.Time
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