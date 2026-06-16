package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func setupIntegrationTestDB(t *testing.T) (*gorm.DB, *security.CryptoEngine, *persistence.GormRepository) {
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

func TestWhatsAppWebhook_HandleWebhook_Unregistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupIntegrationTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	webhook := NewWhatsAppWebhook(db, repo, auditLogger)

	r := gin.New()
	r.POST("/webhook", webhook.HandleWebhook(""))

	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("From", "whatsapp:+19999999999")
	form.Set("Body", "BP:120/80")
	form.Set("NumMedia", "0")

	req, _ := http.NewRequest(http.MethodPost, "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Your phone number is not registered")

	// Verify Auth Failure audit log was generated
	assert.Contains(t, logBuf.String(), "AUTH_FAIL")
}

func TestWhatsAppWebhook_HandleWebhook_RegisterAndParse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupIntegrationTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	webhook := NewWhatsAppWebhook(db, repo, auditLogger)

	// Seed EmergencyQR record to link phone number "+15551234" to patient (SubjectID 42)
	qr := &model.EmergencyQR{
		SubjectID:    42,
		QRHash:       "active-hash-123",
		BloodGroup:   "O+",
		Allergies:    "Nuts",
		ActiveMeds:   "Med A 5mg",
		SponsorPhone: "+15551234",
		IsActive:     true,
	}
	err := repo.RegisterQR(context.Background(), qr)
	require.NoError(t, err)

	r := gin.New()
	r.POST("/webhook", webhook.HandleWebhook(""))

	// 1. Test vital signs parsing text message
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("From", "whatsapp:+15551234")
	form.Set("Body", "Hey doctor, recorded my vitals. BP: 130/85, HR: 72, temp: 98.6")
	form.Set("NumMedia", "0")

	req, _ := http.NewRequest(http.MethodPost, "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Successfully recorded 3 health update(s)")

	// Verify vitals stored in DB
	vitals, err := repo.ListVitals(context.Background(), 42, "", 10)
	require.NoError(t, err)
	assert.Len(t, vitals, 3)

	var bp, hr, temp *model.VitalTelemetry
	for i := range vitals {
		v := &vitals[i]
		switch v.VitalType {
		case "blood_pressure":
			bp = v
		case "heart_rate":
			hr = v
		case "temperature":
			temp = v
		}
	}

	require.NotNil(t, bp)
	assert.Equal(t, 130.0, bp.ValueMetric)
	assert.Equal(t, "mmHg", bp.ValueUnit)
	assert.Contains(t, bp.ContextData, `"diastolic":85`)

	require.NotNil(t, hr)
	assert.Equal(t, 72.0, hr.ValueMetric)
	assert.Equal(t, "bpm", hr.ValueUnit)

	require.NotNil(t, temp)
	assert.Equal(t, 98.6, temp.ValueMetric)
	assert.Equal(t, "F", temp.ValueUnit)

	// Verify Write audit logs
	assert.Contains(t, logBuf.String(), "WRITE_PHI")
	assert.Contains(t, logBuf.String(), "VitalTelemetry")
}

func TestWhatsAppWebhook_HandleWebhook_PrescriptionMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, repo := setupIntegrationTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	webhook := NewWhatsAppWebhook(db, repo, auditLogger)

	// Seed EmergencyQR
	qr := &model.EmergencyQR{
		SubjectID:    101,
		QRHash:       "active-hash-456",
		BloodGroup:   "AB-",
		SponsorPhone: "+15559999",
		IsActive:     true,
	}
	err := repo.RegisterQR(context.Background(), qr)
	require.NoError(t, err)

	r := gin.New()
	r.POST("/webhook", webhook.HandleWebhook(""))

	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("From", "+15559999")
	form.Set("NumMedia", "1")
	form.Set("MediaUrl0", "https://api.twilio.com/prescription.pdf")
	form.Set("MediaContentType0", "application/pdf")

	req, _ := http.NewRequest(http.MethodPost, "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Successfully recorded 1 health update(s)")

	// Verify vital telemetry of type prescription was saved
	vitals, err := repo.ListVitals(context.Background(), 101, "prescription", 1)
	require.NoError(t, err)
	require.Len(t, vitals, 1)
	assert.Equal(t, "prescription", vitals[0].VitalType)
	assert.Equal(t, "url", vitals[0].ValueUnit)

	var ctxMap map[string]interface{}
	err = json.Unmarshal([]byte(vitals[0].ContextData), &ctxMap)
	require.NoError(t, err)
	assert.Equal(t, "https://api.twilio.com/prescription.pdf", ctxMap["media_url"])
	assert.Equal(t, "application/pdf", ctxMap["content_type"])

	// Verify Audit Record
	assert.Contains(t, logBuf.String(), "WRITE_PHI")
	assert.Contains(t, logBuf.String(), "Prescription")
}

func TestVerifyTwilioSignature(t *testing.T) {
	authToken := "super-secret-auth-token"
	signature := "gS7i2p3g21z+h4Vn0C4nBf5ZlK4=" // precalculated hmac-sha1
	reqURL := "https://mycompany.com/webhook"
	params := map[string]string{
		"From":     "+15551111",
		"Body":     "Hello",
		"NumMedia": "0",
	}

	// Calculate manually to confirm VerifyTwilioSignature
	// Sorted: Body=Hello, From=+15551111, NumMedia=0
	// Concatenated: https://mycompany.com/webhookBodyHelloFrom+15551111NumMedia0
	// Signature should match
	isValid := VerifyTwilioSignature(authToken, signature, reqURL, params)
	assert.True(t, isValid)

	// Invalid token / signature
	isValid = VerifyTwilioSignature(authToken, "wrong-signature", reqURL, params)
	assert.False(t, isValid)
}
