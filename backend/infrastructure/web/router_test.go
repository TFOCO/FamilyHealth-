package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	mock_database "github.com/fastenhealth/fasten-onprem/backend/pkg/database/mock"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// In-line mock implementations for the domain repositories
type stubTelemetryRepo struct {
	repository.TelemetryRepository
	recordVitalsFunc func(ctx context.Context, vitals *model.VitalTelemetry) error
	listVitalsFunc   func(ctx context.Context, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error)
}

func (s *stubTelemetryRepo) RecordVitals(ctx context.Context, vitals *model.VitalTelemetry) error {
	if s.recordVitalsFunc != nil {
		return s.recordVitalsFunc(ctx, vitals)
	}
	return nil
}

func (s *stubTelemetryRepo) ListVitals(ctx context.Context, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error) {
	if s.listVitalsFunc != nil {
		return s.listVitalsFunc(ctx, subjectID, vitalType, limit)
	}
	return []model.VitalTelemetry{}, nil
}

type stubFamilyRepo struct {
	repository.FamilyRepository
	getLinkFunc func(ctx context.Context, sponsorID, subjectID uint) (*model.FamilyLink, error)
}

func (s *stubFamilyRepo) GetLink(ctx context.Context, sponsorID, subjectID uint) (*model.FamilyLink, error) {
	if s.getLinkFunc != nil {
		return s.getLinkFunc(ctx, sponsorID, subjectID)
	}
	return &model.FamilyLink{SponsorID: sponsorID, SubjectID: subjectID, AccessRole: "admin"}, nil
}

type stubEmergencyRepo struct {
	repository.EmergencyRepository
	resolveQRFunc func(ctx context.Context, qrHash string) (*model.EmergencyQR, error)
}

func (s *stubEmergencyRepo) ResolveQR(ctx context.Context, qrHash string) (*model.EmergencyQR, error) {
	if s.resolveQRFunc != nil {
		return s.resolveQRFunc(ctx, qrHash)
	}
	return &model.EmergencyQR{SubjectID: 99, IsActive: true, BloodGroup: "O-"}, nil
}

func TestSetupRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mock_database.NewMockDatabaseRepository(ctrl)
	mockConfig := mock_config.NewMockInterface(ctrl)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	telemetryRepo := &stubTelemetryRepo{}
	familyRepo := &stubFamilyRepo{}
	emergencyRepo := &stubEmergencyRepo{}

	router := SetupRouter(mockDB, telemetryRepo, familyRepo, emergencyRepo, mockConfig, auditLogger)

	t.Run("CORS OPTIONS Preflight", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodOptions, "/api/v1/health/telemetry", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})

	t.Run("Public emergency QR code endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/emergency/qr/valid-hash-paramedic?format=json", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "O-")
	})

	t.Run("Blocked telemetry without JWTAuth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/health/telemetry?subject_id=99", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
