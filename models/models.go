package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// User represents the base user/patient in the system
type User struct {
	ID        uint           `gorm:"primaryKey"`
	Username  string         `gorm:"uniqueIndex;not null"`
	Email     string         `gorm:"uniqueIndex;not null"`
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
	OrganizationID uint              `gorm:"not null;index"` // Foreign key to Organization
	StartTime      time.Time         `gorm:"not null"`
	EndTime        time.Time
	Status         AppointmentStatus `gorm:"type:varchar(20);default:'pending'"`
	Notes          string            `gorm:"type:text"` // For patient symptoms or doctor notes
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Relationships
	Patient      User         `gorm:"foreignKey:PatientID"`
	Organization Organization `gorm:"foreignKey:OrganizationID"`
}