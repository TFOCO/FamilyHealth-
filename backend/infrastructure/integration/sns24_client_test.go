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

func TestSNS24Client_GetCMDAuthURL(t *testing.T) {
	client := NewSNS24Client("https://sns24-mock.gov.pt", "client-id-xyz", "secret", "https://app/callback", nil, nil)
	urlStr := client.GetCMDAuthURL("state-123")
	assert.Contains(t, urlStr, "https://sns24-mock.gov.pt/oauth/authorize")
	assert.Contains(t, urlStr, "client_id=client-id-xyz")
	assert.Contains(t, urlStr, "redirect_uri=https%3A%2F%2Fapp%2Fcallback")
	assert.Contains(t, urlStr, "state=state-123")
	assert.Contains(t, urlStr, "scope=openid+profile+sns")
}

func TestSNS24Client_ExchangeCMDCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		_ = r.ParseForm()
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "auth-code-123", r.Form.Get("code"))
		assert.Equal(t, "client-id-xyz", r.Form.Get("client_id"))
		assert.Equal(t, "secret", r.Form.Get("client_secret"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "access-tok-999",
			"token_type": "Bearer",
			"expires_in": 3600,
			"refresh_token": "refresh-tok-777",
			"id_token": "id-tok-888"
		}`))
	}))
	defer server.Close()

	client := NewSNS24Client(server.URL, "client-id-xyz", "secret", "https://app/callback", nil, nil)
	tokenResp, err := client.ExchangeCMDCode(context.Background(), "auth-code-123")
	require.NoError(t, err)
	require.NotNil(t, tokenResp)
	assert.Equal(t, "access-tok-999", tokenResp.AccessToken)
	assert.Equal(t, 3600, tokenResp.ExpiresIn)
}

func TestSNS24Client_GetCMDUserInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/userinfo", r.URL.Path)
		assert.Equal(t, "Bearer access-tok-999", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"sub": "subj-123",
			"name": "Maria Silva",
			"sns_number": "123456789",
			"nif": "999999990",
			"email": "maria@example.com"
		}`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewSNS24Client(server.URL, "client-id-xyz", "secret", "https://app/callback", auditLogger, nil)
	userInfo, err := client.GetCMDUserInfo(context.Background(), "access-tok-999")
	require.NoError(t, err)
	require.NotNil(t, userInfo)
	assert.Equal(t, "Maria Silva", userInfo.Name)
	assert.Equal(t, "123456789", userInfo.SNSNumber)
	assert.Equal(t, "999999990", userInfo.NIF)

	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, security.ActionReadPHI, auditRec.Action)
	assert.Equal(t, "CMDUserInfo", auditRec.ResourceType)
}

func TestSNS24Client_DownloadMedicationReceipts_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/utentes/123456789/receitas", r.URL.Path)
		assert.Equal(t, "Bearer access-tok-999", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"receipt_id": "rec-999-001",
				"issue_date": "2026-06-16T07:55:00Z",
				"prescriber_name": "Dr. Joao Santos",
				"institution": "Centro de Saude de Lisboa",
				"items": [
					{
						"active_substance": "Paracetamol",
						"dosage": "1g",
						"quantity": 20,
						"directions": "Every 8 hours as needed",
						"status": "dispensed"
					}
				]
			}
		]`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewSNS24Client(server.URL, "client-id-xyz", "secret", "https://app/callback", auditLogger, nil)
	receipts, err := client.DownloadMedicationReceipts(context.Background(), 10, 20, "123456789", "access-tok-999")
	require.NoError(t, err)
	require.Len(t, receipts, 1)

	assert.Equal(t, "rec-999-001", receipts[0].ReceiptID)
	assert.Equal(t, "Dr. Joao Santos", receipts[0].PrescriberName)
	require.Len(t, receipts[0].Items, 1)
	assert.Equal(t, "Paracetamol", receipts[0].Items[0].ActiveSubstance)
	assert.Equal(t, 20, receipts[0].Items[0].Quantity)

	// Verify Audit Logging
	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, uint(10), auditRec.OperatorID)
	assert.Equal(t, uint(20), auditRec.TargetPatientID)
	assert.Equal(t, security.ActionReadPHI, auditRec.Action)
	assert.Equal(t, "MedicationReceipts", auditRec.ResourceType)
}

func TestSNS24Client_DownloadVaccinationRecords_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/utentes/123456789/vacinas", r.URL.Path)
		assert.Equal(t, "Bearer access-tok-999", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"vaccine_name": "Comirnaty (COVID-19)",
				"dose_number": 3,
				"date_administered": "2026-06-16T07:55:00Z",
				"manufacturer": "BioNTech/Pfizer",
				"batch_number": "EX1234",
				"next_dose_date": "2027-06-16T07:55:00Z"
			}
		]`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	client := NewSNS24Client(server.URL, "client-id-xyz", "secret", "https://app/callback", auditLogger, nil)
	records, err := client.DownloadVaccinationRecords(context.Background(), 10, 20, "123456789", "access-tok-999")
	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.Equal(t, "Comirnaty (COVID-19)", records[0].VaccineName)
	assert.Equal(t, 3, records[0].DoseNumber)
	assert.Equal(t, "EX1234", records[0].BatchNumber)
	require.NotNil(t, records[0].NextDoseDate)
	assert.Equal(t, time.Date(2027, 6, 16, 7, 55, 0, 0, time.UTC), records[0].NextDoseDate.UTC())

	// Verify Audit Logging
	var auditRec security.AuditRecord
	err = json.Unmarshal(logBuf.Bytes(), &auditRec)
	require.NoError(t, err)
	assert.Equal(t, uint(10), auditRec.OperatorID)
	assert.Equal(t, uint(20), auditRec.TargetPatientID)
	assert.Equal(t, security.ActionReadPHI, auditRec.Action)
	assert.Equal(t, "VaccinationRecords", auditRec.ResourceType)
}
