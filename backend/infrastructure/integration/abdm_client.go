package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// ABDMClient interacts with the ABDM (Ayushman Bharat Digital Mission) sandbox API.
type ABDMClient struct {
	BaseURL     string
	HTTPClient  *http.Client
	AuditLogger *security.AuditLogger
}

// ABHAProfile represents the verified identity profile from Aadhaar verification.
type ABHAProfile struct {
	HealthIDNumber string `json:"healthIdNumber"`
	HealthID       string `json:"healthId"`
	Name           string `json:"name"`
	Gender         string `json:"gender"`
	DateOfBirth    string `json:"dateOfBirth"`
	Mobile         string `json:"mobile"`
	Address        string `json:"address"`
	StateName      string `json:"stateName"`
	DistrictName   string `json:"districtName"`
}

// ABDMError represents standard error responses returned by the ABDM gateway.
type ABDMError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ABDMError) Error() string {
	return fmt.Sprintf("ABDM error (code %d): %s", e.Code, e.Message)
}

// GenerateOTPRequest represents the schema to request Aadhaar OTP.
type GenerateOTPRequest struct {
	Aadhaar string `json:"aadhaar"`
}

// GenerateOTPResponse represents the ABDM response schema containing a transaction ID.
type GenerateOTPResponse struct {
	TransactionID string     `json:"transactionId"`
	Error         *ABDMError `json:"error,omitempty"`
}

// VerifyOTPRequest represents the OTP verification payload.
type VerifyOTPRequest struct {
	TransactionID string `json:"transactionId"`
	OTP           string `json:"otp"`
}

// VerifyOTPResponse represents the verification output containing JWT token and ABHA profile.
type VerifyOTPResponse struct {
	Token       string       `json:"token"`
	ABHAProfile *ABHAProfile `json:"abhaProfile"`
	Error       *ABDMError   `json:"error,omitempty"`
}

// Webhook structures
type ConsentWebhookPayload struct {
	RequestID string               `json:"requestId"`
	Timestamp string               `json:"timestamp"`
	Consent   *ConsentNotification `json:"notification"`
}

// ConsentNotification represents nested consent info
type ConsentNotification struct {
	ConsentRequestID string         `json:"consentRequestId"`
	Status           string         `json:"status"` // GRANTED, DENIED, REVOKED, EXPIRED
	ConsentID        string         `json:"consentId,omitempty"`
	ConsentDetail    *ConsentDetail `json:"consentDetail,omitempty"`
	Signature        string         `json:"signature,omitempty"`
}

type ConsentDetail struct {
	SchemaVersion string            `json:"schemaVersion"`
	ConsentID     string            `json:"consentId"`
	CreatedAt     string            `json:"createdAt"`
	Patient       PatientReference  `json:"patient"`
	HIU           EntityReference   `json:"hiu"`
	HIP           EntityReference   `json:"hip,omitempty"`
	Requester     RequesterInfo     `json:"requester"`
	Purpose       PurposeInfo       `json:"purpose"`
	HITypes       []string          `json:"hiTypes"`
	Permission    ConsentPermission `json:"permission"`
}

type PatientReference struct {
	ID string `json:"id"`
}

type EntityReference struct {
	ID string `json:"id"`
}

type RequesterInfo struct {
	Name       string             `json:"name"`
	Identifier *RequesterIdentity `json:"identifier,omitempty"`
}

type RequesterIdentity struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	System string `json:"system"`
}

type PurposeInfo struct {
	Text string `json:"text"`
	Code string `json:"code"`
}

type ConsentPermission struct {
	AccessMode  string         `json:"accessMode"`
	DateRange   DateRange      `json:"dateRange"`
	DataEraseAt string         `json:"dataEraseAt"`
	Frequency   PermissionFreq `json:"frequency"`
}

type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PermissionFreq struct {
	Unit    string `json:"unit"`
	Value   int    `json:"value"`
	Repeats int    `json:"repeats"`
}

// FHIR Structures for Telemetry Bundle parsing
type FHIRBundle struct {
	ResourceType string      `json:"resourceType"`
	Type         string      `json:"type"`
	Entry        []FHIREntry `json:"entry"`
}

type FHIREntry struct {
	Resource map[string]interface{} `json:"resource"`
}

type FHIRObservation struct {
	ResourceType      string                     `json:"resourceType"`
	ID                string                     `json:"id"`
	Status            string                     `json:"status"`
	Code              FHIRCodeableConcept        `json:"code"`
	Subject           *FHIRReference             `json:"subject,omitempty"`
	EffectiveDateTime string                     `json:"effectiveDateTime"`
	ValueQuantity     *FHIRQuantity              `json:"valueQuantity,omitempty"`
	Component         []FHIRObservationComponent `json:"component,omitempty"`
}

type FHIRCodeableConcept struct {
	Coding []FHIRCoding `json:"coding"`
	Text   string       `json:"text"`
}

type FHIRCoding struct {
	System  string `json:"system"`
	Code    string `json:"code"`
	Display string `json:"display"`
}

type FHIRReference struct {
	Reference string `json:"reference"`
}

type FHIRQuantity struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type FHIRObservationComponent struct {
	Code          FHIRCodeableConcept `json:"code"`
	ValueQuantity *FHIRQuantity       `json:"valueQuantity,omitempty"`
}

// NewABDMClient initializes a new client for ABDM Integration.
func NewABDMClient(baseURL string, auditLogger *security.AuditLogger, httpClient *http.Client) *ABDMClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &ABDMClient{
		BaseURL:     baseURL,
		HTTPClient:  httpClient,
		AuditLogger: auditLogger,
	}
}

// getAuditMeta extracts IP and User Agent from Context if present.
func getAuditMeta(ctx context.Context) (string, string) {
	ip, _ := ctx.Value("ip").(string)
	if ip == "" {
		ip = "127.0.0.1"
	}
	ua, _ := ctx.Value("user_agent").(string)
	if ua == "" {
		ua = "FamilyHealth-Client/1.0"
	}
	return ip, ua
}

// GenerateAadhaarOTP initiates the Aadhaar-based authentication by sending an OTP.
func (c *ABDMClient) GenerateAadhaarOTP(ctx context.Context, operatorID uint, aadhaarNum string) (string, error) {
	ip, ua := getAuditMeta(ctx)

	// Validate Aadhaar Length (Must be 12 digits)
	if len(aadhaarNum) != 12 {
		return "", errors.New("invalid Aadhaar number: must be exactly 12 digits")
	}

	reqBody := GenerateOTPRequest{
		Aadhaar: aadhaarNum,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OTP request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/auth/generateOtp", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var abdmErr ABDMError
		if json.Unmarshal(respBytes, &abdmErr) == nil && abdmErr.Message != "" {
			if c.AuditLogger != nil {
				_ = c.AuditLogger.Log(operatorID, security.ActionAuthFail, 0, "AadhaarOTP", ip, ua)
			}
			return "", &abdmErr
		}
		return "", fmt.Errorf("ABDM Gateway returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var otpResp GenerateOTPResponse
	if err := json.Unmarshal(respBytes, &otpResp); err != nil {
		return "", fmt.Errorf("failed to parse OTP response: %w", err)
	}

	if otpResp.Error != nil {
		if c.AuditLogger != nil {
			_ = c.AuditLogger.Log(operatorID, security.ActionAuthFail, 0, "AadhaarOTP", ip, ua)
		}
		return "", otpResp.Error
	}

	// Compliance audit log for accessing identity resource
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(operatorID, security.ActionReadPHI, 0, "AadhaarOTP", ip, ua)
	}

	return otpResp.TransactionID, nil
}

// VerifyAadhaarOTP verifies the OTP code against a transaction ID.
func (c *ABDMClient) VerifyAadhaarOTP(ctx context.Context, operatorID uint, transactionID string, otp string) (string, *ABHAProfile, error) {
	ip, ua := getAuditMeta(ctx)

	if transactionID == "" {
		return "", nil, errors.New("transaction ID is required")
	}
	if otp == "" {
		return "", nil, errors.New("OTP is required")
	}

	reqBody := VerifyOTPRequest{
		TransactionID: transactionID,
		OTP:           otp,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal verify OTP request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/auth/confirmWithAadhaarOtp", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var abdmErr ABDMError
		if json.Unmarshal(respBytes, &abdmErr) == nil && abdmErr.Message != "" {
			if c.AuditLogger != nil {
				_ = c.AuditLogger.Log(operatorID, security.ActionAuthFail, 0, "AadhaarVerify", ip, ua)
			}
			return "", nil, &abdmErr
		}
		return "", nil, fmt.Errorf("ABDM Gateway returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var verifyResp VerifyOTPResponse
	if err := json.Unmarshal(respBytes, &verifyResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse verify OTP response: %w", err)
	}

	if verifyResp.Error != nil {
		if c.AuditLogger != nil {
			_ = c.AuditLogger.Log(operatorID, security.ActionAuthFail, 0, "AadhaarVerify", ip, ua)
		}
		return "", nil, verifyResp.Error
	}

	// Compliance audit log: PHI details read successfully upon login
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(operatorID, security.ActionReadPHI, 0, "ABHAProfile", ip, ua)
	}

	return verifyResp.Token, verifyResp.ABHAProfile, nil
}

// ParseConsentWebhook parses consent update webhook notifications.
func (c *ABDMClient) ParseConsentWebhook(payloadBytes []byte) (*ConsentWebhookPayload, error) {
	var payload ConsentWebhookPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consent webhook payload: %w", err)
	}
	if payload.RequestID == "" {
		return nil, fmt.Errorf("invalid consent webhook: missing requestId")
	}
	if payload.Consent == nil {
		return nil, fmt.Errorf("invalid consent webhook: missing notification")
	}

	// Compliance audit log for webhook activity
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(0, security.ActionWritePHI, 0, "ConsentManagerUpdate", "0.0.0.0", "ABDM-Webhook")
	}

	return &payload, nil
}

// ParseTelemetryBundle extracts vital signs time-series records from ABDM FHIR bundle.
func (c *ABDMClient) ParseTelemetryBundle(bundleJSON []byte) ([]model.VitalTelemetry, error) {
	var bundle FHIRBundle
	if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal FHIR bundle: %w", err)
	}

	var vitals []model.VitalTelemetry

	for _, entry := range bundle.Entry {
		if entry.Resource == nil {
			continue
		}
		resType, ok := entry.Resource["resourceType"].(string)
		if !ok || resType != "Observation" {
			continue
		}

		obsBytes, err := json.Marshal(entry.Resource)
		if err != nil {
			continue
		}

		var obs FHIRObservation
		if err := json.Unmarshal(obsBytes, &obs); err != nil {
			continue
		}

		var recordedAt time.Time
		if obs.EffectiveDateTime != "" {
			recordedAt, _ = time.Parse(time.RFC3339, obs.EffectiveDateTime)
		}
		if recordedAt.IsZero() {
			recordedAt = time.Now().UTC()
		}

		isBP := false
		isBG := false
		isHR := false

		for _, coding := range obs.Code.Coding {
			switch coding.Code {
			case "8867-4":
				isHR = true
			case "15074-8", "2339-0", "GLUCOSE":
				isBG = true
			case "85354-9", "55284-4", "BP":
				isBP = true
			}
		}

		if isHR && obs.ValueQuantity != nil {
			vitals = append(vitals, model.VitalTelemetry{
				VitalType:   "heart_rate",
				ValueMetric: obs.ValueQuantity.Value,
				ValueUnit:   obs.ValueQuantity.Unit,
				RecordedAt:  recordedAt,
			})
		} else if isBG && obs.ValueQuantity != nil {
			vitals = append(vitals, model.VitalTelemetry{
				VitalType:   "blood_glucose",
				ValueMetric: obs.ValueQuantity.Value,
				ValueUnit:   obs.ValueQuantity.Unit,
				RecordedAt:  recordedAt,
			})
		} else if isBP {
			var systolic, diastolic float64
			var unit string
			for _, comp := range obs.Component {
				for _, coding := range comp.Code.Coding {
					if coding.Code == "8480-6" { // Systolic
						if comp.ValueQuantity != nil {
							systolic = comp.ValueQuantity.Value
							unit = comp.ValueQuantity.Unit
						}
					} else if coding.Code == "8462-4" { // Diastolic
						if comp.ValueQuantity != nil {
							diastolic = comp.ValueQuantity.Value
						}
					}
				}
			}

			if systolic > 0 && diastolic > 0 {
				ctxData := fmt.Sprintf(`{"systolic":%.1f,"diastolic":%.1f}`, systolic, diastolic)
				vitals = append(vitals, model.VitalTelemetry{
					VitalType:   "blood_pressure",
					ValueMetric: systolic,
					ValueUnit:   unit,
					ContextData: ctxData,
					RecordedAt:  recordedAt,
				})
			}
		}
	}

	return vitals, nil
}
