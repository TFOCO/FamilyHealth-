package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/persistence"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupControllerTestDB(t *testing.T) (*gorm.DB, *security.CryptoEngine, *persistence.GormRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	hexKey := "6368616e676570617373776f72646d7573746265333262797465736b65792121"
	crypto, err := security.NewCryptoEngine(hexKey)
	require.NoError(t, err)

	repo := persistence.NewGormRepository(db, crypto)
	err = repo.Migrate()
	require.NoError(t, err)

	return db, crypto, repo
}

func TestDoctorPortalController_ResolveAccessCode_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupControllerTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewDoctorPortalController(repo, repo, auditLogger)

	// Seed QR profile (subject 99)
	qr := &model.EmergencyQR{
		SubjectID:    99,
		QRHash:       "valid-hash-99",
		BloodGroup:   "B+",
		Allergies:    "Sulfa",
		ActiveMeds:   "Medication X 10mg, Medication Y 20mg",
		SponsorPhone: "+15550000",
		IsActive:     true,
	}
	err := repo.RegisterQR(context.Background(), qr)
	require.NoError(t, err)

	// Seed historical vitals
	v1 := &model.VitalTelemetry{
		SubjectID:   99,
		VitalType:   "blood_pressure",
		ValueMetric: 120,
		ValueUnit:   "mmHg",
		ContextData: `{"diastolic":80}`,
		RecordedAt:  time.Now().Add(-10 * time.Minute),
	}
	v2 := &model.VitalTelemetry{
		SubjectID:   99,
		VitalType:   "heart_rate",
		ValueMetric: 70,
		ValueUnit:   "bpm",
		ContextData: `{}`,
		RecordedAt:  time.Now().Add(-5 * time.Minute),
	}
	require.NoError(t, repo.RecordVitals(context.Background(), v1))
	require.NoError(t, repo.RecordVitals(context.Background(), v2))

	r := gin.New()
	r.GET("/portal/:hash", ctrl.ResolveAccessCode)

	// 1. JSON Request via Query Param format=json
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/portal/valid-hash-99?format=json", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "B+", resp["blood_group"])
	assert.Equal(t, "Sulfa", resp["allergies"])
	assert.Equal(t, "Medication X 10mg, Medication Y 20mg", resp["active_meds"])

	vitalsList := resp["vitals"].([]interface{})
	assert.Len(t, vitalsList, 2)

	// Verify PHI Access Log
	assert.Contains(t, logBuf.String(), "READ_PHI")
	assert.Contains(t, logBuf.String(), "DoctorPortal/EmergencyQR")
}

func TestDoctorPortalController_ResolveAccessCode_HTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupControllerTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewDoctorPortalController(repo, repo, auditLogger)

	// Seed QR profile (subject 100)
	qr := &model.EmergencyQR{
		SubjectID:    100,
		QRHash:       "valid-hash-100",
		BloodGroup:   "A-",
		Allergies:    "None",
		ActiveMeds:   "None",
		SponsorPhone: "+15551111",
		IsActive:     true,
	}
	err := repo.RegisterQR(context.Background(), qr)
	require.NoError(t, err)

	// Seed vital
	v1 := &model.VitalTelemetry{
		SubjectID:   100,
		VitalType:   "temperature",
		ValueMetric: 98.4,
		ValueUnit:   "F",
		ContextData: `{}`,
		RecordedAt:  time.Now(),
	}
	require.NoError(t, repo.RecordVitals(context.Background(), v1))

	r := gin.New()
	r.GET("/portal/:hash", ctrl.ResolveAccessCode)

	// 2. HTML Request via Header Accept: text/html
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/portal/valid-hash-100", nil)
	req.Header.Set("Accept", "text/html")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "Emergency Medical Profile")
	assert.Contains(t, bodyStr, "A-")
	assert.Contains(t, bodyStr, "None")
	assert.Contains(t, bodyStr, "Temperature")
	assert.Contains(t, bodyStr, "98.4 F")
}

func TestDoctorPortalController_ResolveAccessCode_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupControllerTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewDoctorPortalController(repo, repo, auditLogger)

	// Seed inactive QR profile
	qr := &model.EmergencyQR{
		SubjectID:    200,
		QRHash:       "expired-hash-200",
		BloodGroup:   "O-",
		SponsorPhone: "+15552222",
		IsActive:     false,
	}
	err := repo.RegisterQR(context.Background(), qr)
	require.NoError(t, err)

	r := gin.New()
	r.GET("/portal/:hash", ctrl.ResolveAccessCode)

	// Request inactive QR hash
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/portal/expired-hash-200", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusForbidden, w1.Code)
	assert.Contains(t, logBuf.String(), "AUTH_FAIL")

	// Request non-existent QR hash
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/portal/does-not-exist", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}
