package repository

import (
	"context"
	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
)

// FamilyRepository defines the data access contract for family links.
type FamilyRepository interface {
	CreateLink(ctx context.Context, link *model.FamilyLink) error
	GetLink(ctx context.Context, sponsorID, subjectID uint) (*model.FamilyLink, error)
	ListLinksBySponsor(ctx context.Context, sponsorID uint) ([]model.FamilyLink, error)
	DeleteLink(ctx context.Context, sponsorID, subjectID uint) error
}

// TelemetryRepository defines the data access contract for vitals telemetry.
type TelemetryRepository interface {
	RecordVitals(ctx context.Context, vitals *model.VitalTelemetry) error
	ListVitals(ctx context.Context, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error)
}

// EmergencyRepository defines the data access contract for emergency QR mapping.
type EmergencyRepository interface {
	RegisterQR(ctx context.Context, qr *model.EmergencyQR) error
	ResolveQR(ctx context.Context, qrHash string) (*model.EmergencyQR, error)
}

// StreakRepository defines the data access contract for user streaks.
type StreakRepository interface {
	GetStreak(ctx context.Context, userID uint) (*model.UserStreak, error)
	SaveStreak(ctx context.Context, streak *model.UserStreak) error
}
