package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABDMClient_GenerateAadhaarOTP_Success(t *testing.T) {
	// Set up mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/auth/generateOtp", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body GenerateOTPRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "123456789012", body.Aadhaar)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"transactionId": "txn-abc-123"}`))
	}))
	defer server.Close()

	// Set up Audit Logger writing to buffer
	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewABDMClient(server.URL, auditLogger, nil)
	ctx := context.WithValue(context.Background(), "ip", "192.168.1.1")
	ctx = context.WithValue(ctx, "user_agent", "TestAgent")

	txnID, err := client.GenerateAadhaarOTP(ctx, 42, "123456789012")
	require.NoError(t, err)
	assert.Equal(t, "txn-abc-123", txnID)

	// Verify Audit Record
	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, uint(42), auditRec.OperatorID)
	assert.Equal(t, security.ActionReadPHI, auditRec.Action)
	assert.Equal(t, "AadhaarOTP", auditRec.ResourceType)
	assert.Equal(t, "192.168.1.1", auditRec.IPAddress)
	assert.Equal(t, "TestAgent", auditRec.UserAgent)
}

func TestABDMClient_GenerateAadhaarOTP_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code": 1001, "message": "Aadhaar number invalid"}`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewABDMClient(server.URL, auditLogger, nil)
	ctx := context.Background()

	_, err := client.GenerateAadhaarOTP(ctx, 42, "123456789012")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Aadhaar number invalid")

	// Verify AuthFail Audit Record
	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, security.ActionAuthFail, auditRec.Action)
}

func TestABDMClient_VerifyAadhaarOTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/auth/confirmWithAadhaarOtp", r.URL.Path)

		var body VerifyOTPRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "txn-abc-123", body.TransactionID)
		assert.Equal(t, "123456", body.OTP)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"token": "jwt-token-xyz",
			"abhaProfile": {
				"healthIdNumber": "12-3456-7890-1234",
				"healthId": "rajesh@sbx",
				"name": "Rajesh Kumar",
				"gender": "M",
				"dateOfBirth": "1980-05-15",
				"mobile": "9999999999"
			}
		}`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewABDMClient(server.URL, auditLogger, nil)
	ctx := context.Background()

	token, profile, err := client.VerifyAadhaarOTP(ctx, 42, "txn-abc-123", "123456")
	require.NoError(t, err)
	assert.Equal(t, "jwt-token-xyz", token)
	require.NotNil(t, profile)
	assert.Equal(t, "12-3456-7890-1234", profile.HealthIDNumber)
	assert.Equal(t, "Rajesh Kumar", profile.Name)

	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, security.ActionReadPHI, auditRec.Action)
	assert.Equal(t, "ABHAProfile", auditRec.ResourceType)
}

func TestABDMClient_ParseConsentWebhook(t *testing.T) {
	payload := `{
		"requestId": "req-uuid-111",
		"timestamp": "2026-06-16T07:55:00Z",
		"notification": {
			"consentRequestId": "consent-req-777",
			"status": "GRANTED",
			"consentId": "consent-id-999",
			"consentDetail": {
				"schemaVersion": "1.0",
				"consentId": "consent-id-999",
				"createdAt": "2026-06-16T07:55:00Z",
				"patient": { "id": "rajesh@sbx" },
				"hiu": { "id": "hiu-id-1" },
				"requester": { "name": "Dr. Rajesh" },
				"purpose": { "text": "Self-Monitoring", "code": "PATRQT" },
				"hiTypes": ["Observation"],
				"permission": {
					"accessMode": "VIEW",
					"dateRange": {
						"from": "2020-01-01T00:00:00Z",
						"to": "2026-06-16T00:00:00Z"
					},
					"dataEraseAt": "2027-06-16T00:00:00Z",
					"frequency": { "unit": "HOUR", "value": 1, "repeats": 0 }
				}
			}
		}
	}`

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewABDMClient("", auditLogger, nil)
	webhookData, err := client.ParseConsentWebhook([]byte(payload))
	require.NoError(t, err)
	require.NotNil(t, webhookData)
	assert.Equal(t, "req-uuid-111", webhookData.RequestID)
	assert.Equal(t, "GRANTED", webhookData.Consent.Status)
	assert.Equal(t, "rajesh@sbx", webhookData.Consent.ConsentDetail.Patient.ID)
}

func TestABDMClient_ParseTelemetryBundle(t *testing.T) {
	bundleJSON := `{
		"resourceType": "Bundle",
		"type": "searchset",
		"entry": [
			{
				"resource": {
					"resourceType": "Observation",
					"id": "hr-1",
					"status": "final",
					"code": {
						"coding": [
							{
								"system": "http://loinc.org",
								"code": "8867-4",
								"display": "Heart rate"
							}
						]
					},
					"effectiveDateTime": "2026-06-16T07:55:00Z",
					"valueQuantity": {
						"value": 72.0,
						"unit": "bpm"
					}
				}
			},
			{
				"resource": {
					"resourceType": "Observation",
					"id": "bg-1",
					"status": "final",
					"code": {
						"coding": [
							{
								"system": "http://loinc.org",
								"code": "15074-8",
								"display": "Glucose"
							}
						]
					},
					"effectiveDateTime": "2026-06-16T07:56:00Z",
					"valueQuantity": {
						"value": 110.5,
						"unit": "mg/dL"
					}
				}
			},
			{
				"resource": {
					"resourceType": "Observation",
					"id": "bp-1",
					"status": "final",
					"code": {
						"coding": [
							{
								"system": "http://loinc.org",
								"code": "85354-9",
								"display": "Blood pressure"
							}
						]
					},
					"effectiveDateTime": "2026-06-16T07:57:00Z",
					"component": [
						{
							"code": {
								"coding": [
									{
										"system": "http://loinc.org",
										"code": "8480-6",
										"display": "Systolic blood pressure"
									}
								]
							},
							"valueQuantity": {
								"value": 120.0,
								"unit": "mmHg"
							}
						},
						{
							"code": {
								"coding": [
									{
										"system": "http://loinc.org",
										"code": "8462-4",
										"display": "Diastolic blood pressure"
									}
								]
							},
							"valueQuantity": {
								"value": 80.0,
								"unit": "mmHg"
							}
						}
					]
				}
			}
		]
	}`

	client := NewABDMClient("", nil, nil)
	vitals, err := client.ParseTelemetryBundle([]byte(bundleJSON))
	require.NoError(t, err)
	require.Len(t, vitals, 3)

	// Verify Heart Rate
	assert.Equal(t, "heart_rate", vitals[0].VitalType)
	assert.Equal(t, 72.0, vitals[0].ValueMetric)
	assert.Equal(t, "bpm", vitals[0].ValueUnit)
	assert.Equal(t, time.Date(2026, 6, 16, 7, 55, 0, 0, time.UTC), vitals[0].RecordedAt.UTC())

	// Verify Blood Glucose
	assert.Equal(t, "blood_glucose", vitals[1].VitalType)
	assert.Equal(t, 110.5, vitals[1].ValueMetric)
	assert.Equal(t, "mg/dL", vitals[1].ValueUnit)
	assert.Equal(t, time.Date(2026, 6, 16, 7, 56, 0, 0, time.UTC), vitals[1].RecordedAt.UTC())

	// Verify Blood Pressure
	assert.Equal(t, "blood_pressure", vitals[2].VitalType)
	assert.Equal(t, 120.0, vitals[2].ValueMetric)
	assert.Equal(t, "mmHg", vitals[2].ValueUnit)
	assert.JSONEq(t, `{"systolic":120.0,"diastolic":80.0}`, vitals[2].ContextData)
	assert.Equal(t, time.Date(2026, 6, 16, 7, 57, 0, 0, time.UTC), vitals[2].RecordedAt.UTC())
}
