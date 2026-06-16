package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// SNS24Client handles integrations with the Portuguese National Health Service (SNS24) portal.
type SNS24Client struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	AuditLogger  *security.AuditLogger
}

// CMDTokenResponse represents the token response from CMD OAuth exchange.
type CMDTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// CMDUserInfo represents the identity claims returned by CMD.
type CMDUserInfo struct {
	Sub       string `json:"sub"`
	Name      string `json:"name"`
	SNSNumber string `json:"sns_number"` // Utente number
	NIF       string `json:"nif"`        // Portuguese tax number
	Email     string `json:"email"`
}

// MedicationReceipt represents a digital drug prescription.
type MedicationReceipt struct {
	ReceiptID      string           `json:"receipt_id"`
	IssueDate      time.Time        `json:"issue_date"`
	PrescriberName string           `json:"prescriber_name"`
	Institution    string           `json:"institution"`
	Items          []MedicationItem `json:"items"`
}

// MedicationItem represents a single prescribed medication inside a receipt.
type MedicationItem struct {
	ActiveSubstance string `json:"active_substance"`
	Dosage          string `json:"dosage"`
	Quantity        int    `json:"quantity"`
	Directions      string `json:"directions"`
	Status          string `json:"status"` // dispensed, pending
}

// VaccinationRecord represents an immunization receipt or booklet entry.
type VaccinationRecord struct {
	VaccineName      string     `json:"vaccine_name"`
	DoseNumber       int        `json:"dose_number"`
	DateAdministered time.Time  `json:"date_administered"`
	Manufacturer     string     `json:"manufacturer"`
	BatchNumber      string     `json:"batch_number"`
	NextDoseDate     *time.Time `json:"next_dose_date,omitempty"`
}

// SNSError represents standard error structures from the Portuguese SNS24 endpoints.
type SNSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SNSError) Error() string {
	return fmt.Sprintf("SNS24 error (%s): %s", e.Code, e.Message)
}

// NewSNS24Client initializes an SNS24 client.
func NewSNS24Client(baseURL, clientID, clientSecret, redirectURL string, auditLogger *security.AuditLogger, httpClient *http.Client) *SNS24Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &SNS24Client{
		BaseURL:      baseURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		HTTPClient:   httpClient,
		AuditLogger:  auditLogger,
	}
}

// GetCMDAuthURL constructs the authorization URL to redirect users for Chave Móvel Digital login.
func (c *SNS24Client) GetCMDAuthURL(state string) string {
	u, err := url.Parse(c.BaseURL + "/oauth/authorize")
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	q.Set("scope", "openid profile sns")
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeCMDCode exchanges the authorization code for access tokens.
func (c *SNS24Client) ExchangeCMDCode(ctx context.Context, code string) (*CMDTokenResponse, error) {
	ip, ua := getAuditMeta(ctx)

	if code == "" {
		return nil, errors.New("authorization code is required")
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.RedirectURL)
	data.Set("client_id", c.ClientID)
	data.Set("client_secret", c.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if c.AuditLogger != nil {
			_ = c.AuditLogger.Log(0, security.ActionAuthFail, 0, "CMDExchange", ip, ua)
		}
		var snsErr SNSError
		if json.Unmarshal(respBytes, &snsErr) == nil && snsErr.Message != "" {
			return nil, &snsErr
		}
		return nil, fmt.Errorf("CMD OAuth token endpoint returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var tokenResp CMDTokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// GetCMDUserInfo retrieves the user profile information using the CMD access token.
func (c *SNS24Client) GetCMDUserInfo(ctx context.Context, accessToken string) (*CMDUserInfo, error) {
	ip, ua := getAuditMeta(ctx)

	if accessToken == "" {
		return nil, errors.New("access token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/oauth/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute userinfo request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if c.AuditLogger != nil {
			_ = c.AuditLogger.Log(0, security.ActionAuthFail, 0, "CMDUserInfo", ip, ua)
		}
		var snsErr SNSError
		if json.Unmarshal(respBytes, &snsErr) == nil && snsErr.Message != "" {
			return nil, &snsErr
		}
		return nil, fmt.Errorf("CMD UserInfo returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var userInfo CMDUserInfo
	if err := json.Unmarshal(respBytes, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo response: %w", err)
	}

	// Compliance audit log for successful profile retrieval
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(0, security.ActionReadPHI, 0, "CMDUserInfo", ip, ua)
	}

	return &userInfo, nil
}

// DownloadMedicationReceipts retrieves the user's active prescriptions / medication receipts.
func (c *SNS24Client) DownloadMedicationReceipts(ctx context.Context, operatorID uint, patientUserID uint, snsNumber string, token string) ([]MedicationReceipt, error) {
	ip, ua := getAuditMeta(ctx)

	if snsNumber == "" {
		return nil, errors.New("SNS/Utente number is required")
	}
	if token == "" {
		return nil, errors.New("OAuth token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/utentes/%s/receitas", c.BaseURL, snsNumber), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create medication request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute medication request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var snsErr SNSError
		if json.Unmarshal(respBytes, &snsErr) == nil && snsErr.Message != "" {
			return nil, &snsErr
		}
		return nil, fmt.Errorf("medication download returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var receipts []MedicationReceipt
	if err := json.Unmarshal(respBytes, &receipts); err != nil {
		return nil, fmt.Errorf("failed to parse medication receipts: %w", err)
	}

	// Compliance audit log: Reading PHI (MedicationReceipts)
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(operatorID, security.ActionReadPHI, patientUserID, "MedicationReceipts", ip, ua)
	}

	return receipts, nil
}

// DownloadVaccinationRecords retrieves the user's vaccination history.
func (c *SNS24Client) DownloadVaccinationRecords(ctx context.Context, operatorID uint, patientUserID uint, snsNumber string, token string) ([]VaccinationRecord, error) {
	ip, ua := getAuditMeta(ctx)

	if snsNumber == "" {
		return nil, errors.New("SNS/Utente number is required")
	}
	if token == "" {
		return nil, errors.New("OAuth token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/utentes/%s/vacinas", c.BaseURL, snsNumber), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create vaccination request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vaccination request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var snsErr SNSError
		if json.Unmarshal(respBytes, &snsErr) == nil && snsErr.Message != "" {
			return nil, &snsErr
		}
		return nil, fmt.Errorf("vaccination download returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var records []VaccinationRecord
	if err := json.Unmarshal(respBytes, &records); err != nil {
		return nil, fmt.Errorf("failed to parse vaccination records: %w", err)
	}

	// Compliance audit log: Reading PHI (VaccinationRecords)
	if c.AuditLogger != nil {
		_ = c.AuditLogger.Log(operatorID, security.ActionReadPHI, patientUserID, "VaccinationRecords", ip, ua)
	}

	return records, nil
}
