package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	bpRegex      = regexp.MustCompile(`(?i)(?:bp|blood\s*pressure)\s*:\s*(\d+)\s*/\s*(\d+)`)
	glucoseRegex = regexp.MustCompile(`(?i)(?:glucose|sugar)\s*:\s*(\d+(?:\.\d+)?)`)
	hrRegex      = regexp.MustCompile(`(?i)(?:hr|heart\s*rate|pulse)\s*:\s*(\d+(?:\.\d+)?)`)
	tempRegex    = regexp.MustCompile(`(?i)(?:temp|temperature)\s*:\s*(\d+(?:\.\d+)?)`)
)

// VerifyTwilioSignature validates that the request came from Twilio.
func VerifyTwilioSignature(authToken, signature, url string, params map[string]string) bool {
	if authToken == "" {
		return true // skip verification if no auth token is provided (useful for testing/development)
	}

	// 1. Sort parameter keys alphabetically
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Concatenate URL and sorted parameter key-value pairs
	var buf strings.Builder
	buf.WriteString(url)
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(params[k])
	}

	// 3. Compute HMAC-SHA1 signature
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(buf.String()))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// WhatsAppWebhook handles Twilio integration endpoints
type WhatsAppWebhook struct {
	db          *gorm.DB
	telemetry   repository.TelemetryRepository
	auditLogger *security.AuditLogger
}

// NewWhatsAppWebhook creates a new WhatsAppWebhook instance.
func NewWhatsAppWebhook(db *gorm.DB, telemetry repository.TelemetryRepository, auditLogger *security.AuditLogger) *WhatsAppWebhook {
	return &WhatsAppWebhook{
		db:          db,
		telemetry:   telemetry,
		auditLogger: auditLogger,
	}
}

// HandleWebhook returns a Gin handler for parsing incoming Twilio webhooks
func (w *WhatsAppWebhook) HandleWebhook(authToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse form fields
		if err := c.Request.ParseForm(); err != nil {
			c.Header("Content-Type", "application/xml")
			c.String(http.StatusBadRequest, `<?xml version="1.0" encoding="UTF-8"?><Response><Message>Failed to parse form data</Message></Response>`)
			return
		}

		params := make(map[string]string)
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}

		// Verify signature if auth token is set
		if authToken != "" {
			signature := c.GetHeader("X-Twilio-Signature")
			scheme := "http"
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			reqURL := fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, c.Request.URL.RequestURI())

			if !VerifyTwilioSignature(authToken, signature, reqURL, params) {
				_ = w.auditLogger.Log(
					0,
					security.ActionAuthFail,
					0,
					"TwilioWebhook",
					c.ClientIP(),
					c.Request.UserAgent(),
				)
				c.Header("Content-Type", "application/xml")
				c.String(http.StatusForbidden, `<?xml version="1.0" encoding="UTF-8"?><Response><Message>Forbidden: Invalid Twilio signature</Message></Response>`)
				return
			}
		}

		fromVal := c.PostForm("From")
		bodyVal := c.PostForm("Body")
		numMediaStr := c.PostForm("NumMedia")

		if fromVal == "" {
			c.Header("Content-Type", "application/xml")
			c.String(http.StatusBadRequest, `<?xml version="1.0" encoding="UTF-8"?><Response><Message>Bad Request: Missing From number</Message></Response>`)
			return
		}

		// Clean WhatsApp phone number prefix if present
		cleanPhone := strings.TrimPrefix(fromVal, "whatsapp:")
		cleanPhone = strings.TrimSpace(cleanPhone)

		// Find the emergency responder profile link (matching sponsor_phone to find patient)
		type emergencyQRTemp struct {
			SubjectID uint `gorm:"column:subject_id"`
			IsActive  bool `gorm:"column:is_active"`
		}
		var qr emergencyQRTemp
		err := w.db.Table("emergency_qrs").
			Select("subject_id, is_active").
			Where("sponsor_phone = ?", cleanPhone).
			First(&qr).Error

		if err != nil || !qr.IsActive {
			_ = w.auditLogger.Log(
				0,
				security.ActionAuthFail,
				0,
				"TwilioWebhook",
				c.ClientIP(),
				c.Request.UserAgent(),
			)
			c.Header("Content-Type", "application/xml")
			c.String(http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>Your phone number is not registered for emergency updates. Please register it in the FamilyHealth portal first.</Message>
</Response>`)
			return
		}

		subjectID := qr.SubjectID
		var recordedCount int

		// Parse media attachments (prescriptions)
		numMedia, _ := strconv.Atoi(numMediaStr)
		if numMedia > 0 {
			for i := 0; i < numMedia; i++ {
				mediaURL := c.PostForm(fmt.Sprintf("MediaUrl%d", i))
				contentType := c.PostForm(fmt.Sprintf("MediaContentType%d", i))
				if mediaURL != "" {
					contextJSON, _ := json.Marshal(map[string]interface{}{
						"media_url":    mediaURL,
						"content_type": contentType,
					})
					record := model.VitalTelemetry{
						SubjectID:   subjectID,
						VitalType:   "prescription",
						ValueMetric: 0,
						ValueUnit:   "url",
						ContextData: string(contextJSON),
						RecordedAt:  time.Now(),
					}
					if err := w.telemetry.RecordVitals(c.Request.Context(), &record); err == nil {
						recordedCount++
						_ = w.auditLogger.Log(
							0,
							security.ActionWritePHI,
							subjectID,
							"Prescription",
							c.ClientIP(),
							c.Request.UserAgent(),
						)
					}
				}
			}
		}

		// Parse text body for vital signs
		if bodyVal != "" {
			records, err := w.ParseAndSaveVitals(c.Request.Context(), subjectID, bodyVal, c.ClientIP(), c.Request.UserAgent())
			if err == nil {
				recordedCount += len(records)
			}
		}

		c.Header("Content-Type", "application/xml")
		if recordedCount > 0 {
			c.String(http.StatusOK, fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>Successfully recorded %d health update(s) securely.</Message>
</Response>`, recordedCount))
		} else {
			c.String(http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>No recognizable health telemetry (BP, Glucose, Heart Rate, Temp) or prescription media found in your message.</Message>
</Response>`)
		}
	}
}

// ParseAndSaveVitals extracts vital signs from free text and persists them as VitalTelemetry.
func (w *WhatsAppWebhook) ParseAndSaveVitals(ctx context.Context, subjectID uint, body, ip, userAgent string) ([]model.VitalTelemetry, error) {
	var records []model.VitalTelemetry

	// 1. Blood Pressure
	if matches := bpRegex.FindStringSubmatch(body); len(matches) == 3 {
		sys, _ := strconv.ParseFloat(matches[1], 64)
		dia, _ := strconv.ParseFloat(matches[2], 64)
		contextJSON, _ := json.Marshal(map[string]interface{}{
			"diastolic": dia,
			"systolic":  sys,
		})
		records = append(records, model.VitalTelemetry{
			SubjectID:   subjectID,
			VitalType:   "blood_pressure",
			ValueMetric: sys,
			ValueUnit:   "mmHg",
			ContextData: string(contextJSON),
			RecordedAt:  time.Now(),
		})
	}

	// 2. Glucose
	if matches := glucoseRegex.FindStringSubmatch(body); len(matches) == 2 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		records = append(records, model.VitalTelemetry{
			SubjectID:   subjectID,
			VitalType:   "blood_glucose",
			ValueMetric: val,
			ValueUnit:   "mg/dL",
			ContextData: `{}`,
			RecordedAt:  time.Now(),
		})
	}

	// 3. Heart Rate
	if matches := hrRegex.FindStringSubmatch(body); len(matches) == 2 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		records = append(records, model.VitalTelemetry{
			SubjectID:   subjectID,
			VitalType:   "heart_rate",
			ValueMetric: val,
			ValueUnit:   "bpm",
			ContextData: `{}`,
			RecordedAt:  time.Now(),
		})
	}

	// 4. Temperature
	if matches := tempRegex.FindStringSubmatch(body); len(matches) == 2 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		records = append(records, model.VitalTelemetry{
			SubjectID:   subjectID,
			VitalType:   "temperature",
			ValueMetric: val,
			ValueUnit:   "F",
			ContextData: `{}`,
			RecordedAt:  time.Now(),
		})
	}

	// Save records to GORM
	for i := range records {
		err := w.telemetry.RecordVitals(ctx, &records[i])
		if err != nil {
			return nil, fmt.Errorf("failed to record vital telemetry: %w", err)
		}
		_ = w.auditLogger.Log(
			0,
			security.ActionWritePHI,
			subjectID,
			"VitalTelemetry",
			ip,
			userAgent,
		)
	}

	return records, nil
}
