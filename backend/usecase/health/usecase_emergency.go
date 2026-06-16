package health

import (
	"context"
	"errors"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
)

// EmergencyUsecase handles emergency QR scanning and alerting.
type EmergencyUsecase struct {
	emergencyRepo repository.EmergencyRepository
}

// NewEmergencyUsecase creates a new instance.
func NewEmergencyUsecase(emergencyRepo repository.EmergencyRepository) *EmergencyUsecase {
	return &EmergencyUsecase{
		emergencyRepo: emergencyRepo,
	}
}

// ResolveEmergencyQR fetches the profile and triggers alerts.
func (u *EmergencyUsecase) ResolveEmergencyQR(ctx context.Context, qrHash string) (*model.EmergencyQR, error) {
	if qrHash == "" {
		return nil, errors.New("qr_hash is required")
	}

	profile, err := u.emergencyRepo.FindByHash(ctx, qrHash)
	if err != nil {
		return nil, err
	}

	if !profile.IsActive {
		return nil, errors.New("emergency profile is inactive")
	}

	// In a real application, trigger an asynchronous background worker here
	// to send a WhatsApp/SMS alert to profile.SponsorPhone using Twilio or similar.
	// e.g., go notifySponsor(profile.SponsorPhone, "Emergency QR Scanned!")

	return profile, nil
}
