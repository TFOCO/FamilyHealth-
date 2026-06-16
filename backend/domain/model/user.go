package model

import (
	"time"
)

// User represents the core user domain entity.
type User struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	FullName  string    `json:"full_name"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // Never expose password in JSON
	Picture   string    `json:"picture"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // "admin", "viewer", "billing"
}

// FamilyLink represents a parent-child or caregiver-patient linkage.
type FamilyLink struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	SponsorID  uint      `json:"sponsor_id"`
	SubjectID  uint      `json:"subject_id"`
	Relation   string    `json:"relation"`     // e.g. "Father", "Mother"
	AccessRole string    `json:"access_role"`  // e.g. "admin", "viewer"
}

// VitalTelemetry represents time-series biometric details for remote health tracking.
type VitalTelemetry struct {
	ID          uint      `json:"id"`
	SubjectID   uint      `json:"subject_id"`
	VitalType   string    `json:"vital_type"` // e.g. "blood_pressure", "blood_glucose"
	ValueMetric float64   `json:"value_metric"`
	ValueUnit   string    `json:"value_unit"` // e.g. "mmHg", "mg/dL"
	ContextData string    `json:"context_data"` // JSON metadata holding secondary telemetry parameters
	RecordedAt  time.Time `json:"recorded_at"`
}

// EmergencyQR represents a parent's emergency QR profile scanned by responders.
type EmergencyQR struct {
	ID           uint      `json:"id"`
	SubjectID    uint      `json:"subject_id"`
	QRHash       string    `json:"qr_hash"`
	BloodGroup   string    `json:"blood_group"`
	Allergies    string    `json:"allergies"`
	ActiveMeds   string    `json:"active_meds"`
	SponsorPhone string    `json:"sponsor_phone"`
	IsActive     bool      `json:"is_active"`
}

// UserStreak tracks daily logging streaks for a user.
type UserStreak struct {
	ID             uint      `json:"id"`
	UserID         uint      `json:"user_id"`
	CurrentStreak  int       `json:"current_streak"`
	MaxStreak      int       `json:"max_streak"`
	LastLoggedDate time.Time `json:"last_logged_date"`
}

// IncrementStreak updates the streak based on a new log entry.
// It returns true if the current streak was incremented, or false otherwise.
func (s *UserStreak) IncrementStreak(logTime time.Time, loc *time.Location) bool {
	if loc == nil {
		loc = time.UTC
	}

	logTimeLocal := logTime.In(loc)
	today := time.Date(logTimeLocal.Year(), logTimeLocal.Month(), logTimeLocal.Day(), 0, 0, 0, 0, loc)

	if s.LastLoggedDate.IsZero() {
		s.CurrentStreak = 1
		s.MaxStreak = 1
		s.LastLoggedDate = logTime
		return true
	}

	lastLoggedLocal := s.LastLoggedDate.In(loc)
	lastLogDate := time.Date(lastLoggedLocal.Year(), lastLoggedLocal.Month(), lastLoggedLocal.Day(), 0, 0, 0, 0, loc)

	nextExpectedDate := lastLogDate.AddDate(0, 0, 1)

	if today.Equal(lastLogDate) {
		if logTime.After(s.LastLoggedDate) {
			s.LastLoggedDate = logTime
		}
		return false
	}

	if today.Equal(nextExpectedDate) {
		s.CurrentStreak++
		if s.CurrentStreak > s.MaxStreak {
			s.MaxStreak = s.CurrentStreak
		}
		s.LastLoggedDate = logTime
		return true
	}

	if today.After(nextExpectedDate) {
		s.CurrentStreak = 1
		s.LastLoggedDate = logTime
		return true
	}

	return false
}

// ResetStreak resets the current streak back to 0.
func (s *UserStreak) ResetStreak() {
	s.CurrentStreak = 0
}
