package payments

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// EscrowState represents the state of an escrow transaction.
type EscrowState string

const (
	EscrowStatePending   EscrowState = "PENDING"
	EscrowStateHeld      EscrowState = "HELD"
	EscrowStateDisbursed EscrowState = "DISBURSED"
	EscrowStateRefunded  EscrowState = "REFUNDED"
	EscrowStateFailed    EscrowState = "FAILED"
)

// EscrowTransaction represents the lifecycle and records of a cross-border care escrow transaction.
type EscrowTransaction struct {
	ID               string      `json:"id"`
	SponsorID        uint        `json:"sponsor_id"`
	SubjectID        uint        `json:"subject_id"`
	ProviderID       string      `json:"provider_id"`        // Vetted local service provider (e.g. lab, pharmacy)
	AmountIn         float64     `json:"amount_in"`          // Amount received from sponsor (e.g., in USD/EUR)
	CurrencyIn       string      `json:"currency_in"`        // USD/EUR
	AmountOut        float64     `json:"amount_out"`         // Amount disbursed to provider (e.g., in INR)
	CurrencyOut      string      `json:"currency_out"`       // INR
	State            EscrowState `json:"state"`
	GatewayTxID      string      `json:"gateway_tx_id"`      // Reference ID from payment gateway
	DisbursementTxID string      `json:"disbursement_tx_id"` // Reference ID from payout gateway
	ErrorMessage     string      `json:"error_message,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// PaymentResult represents the status of an incoming charge operation.
type PaymentResult struct {
	GatewayTxID string
	Amount      float64
	Currency    string
	Success     bool
	Message     string
}

// DisbursementResult represents the status of an outgoing payout operation.
type DisbursementResult struct {
	DisbursementTxID string
	Amount           float64
	Currency         string
	Success          bool
	Message          string
}

// AuditContext captures security metadata for HIPAA/DPDP compliance logging.
type AuditContext struct {
	OperatorID uint
	IPAddress  string
	UserAgent  string
}

// PaymentGateway defines the contract for processing payments from the Sponsor.
type PaymentGateway interface {
	// Charge processes a card or other payment method charge.
	Charge(ctx context.Context, amount float64, currency string, token string) (*PaymentResult, error)
	// Refund processes a refund for a previously captured charge.
	Refund(ctx context.Context, gatewayTxID string) error
	// GetName returns the payment gateway name.
	GetName() string
}

// UPIPaymentGateway extends PaymentGateway with UPI-specific intent generation.
type UPIPaymentGateway interface {
	PaymentGateway
	GenerateUPIIntent(ctx context.Context, amount float64, payeeVPA, payeeName, txRef, note string) (string, error)
}

// DisbursementGateway defines the contract for paying out funds to local service providers.
type DisbursementGateway interface {
	// Disburse initiates a transfer of funds to a service provider.
	Disburse(ctx context.Context, providerID string, amount float64, currency string) (*DisbursementResult, error)
	// GetName returns the disbursement gateway name.
	GetName() string
}

// EscrowRepository defines the persistence contract for escrow transactions.
type EscrowRepository interface {
	Save(ctx context.Context, tx *EscrowTransaction) error
	Get(ctx context.Context, txID string) (*EscrowTransaction, error)
}

// EscrowService defines the business logic for the care funding escrow lifecycle.
type EscrowService interface {
	// InitiateEscrow processes the sponsor's payment and puts the transaction into the Held state.
	InitiateEscrow(ctx context.Context, auditCtx AuditContext, sponsorID, subjectID uint, providerID string, amountIn float64, currencyIn string, token string) (*EscrowTransaction, error)
	// DisburseEscrow payouts the held funds to a local provider.
	DisburseEscrow(ctx context.Context, auditCtx AuditContext, txID string, currencyOut string, exchangeRate float64) (*EscrowTransaction, error)
	// RefundEscrow refunds the escrowed funds to the sponsor.
	RefundEscrow(ctx context.Context, auditCtx AuditContext, txID string) (*EscrowTransaction, error)
	// GetTransaction retrieves a transaction by ID.
	GetTransaction(ctx context.Context, txID string) (*EscrowTransaction, error)
}

// ============================================================================
// Mock Stripe Client
// ============================================================================

// StripeMockClient mocks the Stripe payment gateway.
type StripeMockClient struct {
	mu             sync.Mutex
	Name           string
	ChargedAmount  float64
	ChargedCount   int
	RefundedCount  int
	SimulateError  error
	SimulateDecline bool
}

var _ PaymentGateway = (*StripeMockClient)(nil)

func NewStripeMockClient() *StripeMockClient {
	return &StripeMockClient{
		Name: "Stripe",
	}
}

func (c *StripeMockClient) Charge(ctx context.Context, amount float64, currency string, token string) (*PaymentResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return nil, c.SimulateError
	}
	if c.SimulateDecline || token == "tok_decline" || token == "fail_charge" {
		return &PaymentResult{
			Success: false,
			Message: "Card declined by issuer",
		}, nil
	}

	c.ChargedAmount += amount
	c.ChargedCount++

	txID := fmt.Sprintf("ch_stripe_%d", time.Now().UnixNano())
	return &PaymentResult{
		GatewayTxID: txID,
		Amount:      amount,
		Currency:    currency,
		Success:     true,
		Message:     "Charge succeeded",
	}, nil
}

func (c *StripeMockClient) Refund(ctx context.Context, gatewayTxID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return c.SimulateError
	}
	if gatewayTxID == "" {
		return errors.New("cannot refund empty gateway transaction ID")
	}

	c.RefundedCount++
	return nil
}

func (c *StripeMockClient) GetName() string {
	return c.Name
}

// ============================================================================
// Mock Razorpay Client
// ============================================================================

// RazorpayMockClient mocks the Razorpay payment and disbursement gateway.
type RazorpayMockClient struct {
	mu                 sync.Mutex
	Name               string
	ChargedAmount      float64
	ChargedCount       int
	DisbursedAmount    float64
	DisbursedCount     int
	SimulateError      error
	SimulateDecline    bool
	SimulatePayoutFail bool
}

var _ PaymentGateway = (*RazorpayMockClient)(nil)
var _ DisbursementGateway = (*RazorpayMockClient)(nil)

func NewRazorpayMockClient() *RazorpayMockClient {
	return &RazorpayMockClient{
		Name: "Razorpay",
	}
}

func (c *RazorpayMockClient) Charge(ctx context.Context, amount float64, currency string, token string) (*PaymentResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return nil, c.SimulateError
	}
	if c.SimulateDecline || token == "rzp_fail" || token == "fail_charge" {
		return &PaymentResult{
			Success: false,
			Message: "Razorpay payment authorization failed",
		}, nil
	}

	c.ChargedAmount += amount
	c.ChargedCount++

	txID := fmt.Sprintf("pay_rzp_%d", time.Now().UnixNano())
	return &PaymentResult{
		GatewayTxID: txID,
		Amount:      amount,
		Currency:    currency,
		Success:     true,
		Message:     "Razorpay charge succeeded",
	}, nil
}

func (c *RazorpayMockClient) Refund(ctx context.Context, gatewayTxID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return c.SimulateError
	}
	if gatewayTxID == "" {
		return errors.New("cannot refund empty gateway transaction ID")
	}

	return nil
}

func (c *RazorpayMockClient) Disburse(ctx context.Context, providerID string, amount float64, currency string) (*DisbursementResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return nil, c.SimulateError
	}
	if c.SimulatePayoutFail || providerID == "fail_provider" {
		return &DisbursementResult{
			Success: false,
			Message: "Payout failed: routing error",
		}, nil
	}

	c.DisbursedAmount += amount
	c.DisbursedCount++

	payoutID := fmt.Sprintf("pout_rzp_%d", time.Now().UnixNano())
	return &DisbursementResult{
		DisbursementTxID: payoutID,
		Amount:           amount,
		Currency:         currency,
		Success:          true,
		Message:          "Payout succeeded",
	}, nil
}

func (c *RazorpayMockClient) GetName() string {
	return c.Name
}

func (c *RazorpayMockClient) GenerateUPIIntent(ctx context.Context, amount float64, payeeVPA, payeeName, txRef, note string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SimulateError != nil {
		return "", c.SimulateError
	}

	// Generate standard, NPIC-compliant UPI payload URI
	// e.g. upi://pay?pa=payee@vpa&pn=PayeeName&tr=txRef&tn=Note&am=Amount&cu=INR
	uri := fmt.Sprintf("upi://pay?pa=%s&pn=%s&tr=%s&tn=%s&am=%.2f&cu=INR",
		payeeVPA, payeeName, txRef, note, amount)
	return uri, nil
}

// ============================================================================
// In-Memory Escrow Repository
// ============================================================================

// InMemoryEscrowRepository implements EscrowRepository.
type InMemoryEscrowRepository struct {
	mu  sync.RWMutex
	txs map[string]*EscrowTransaction
}

var _ EscrowRepository = (*InMemoryEscrowRepository)(nil)

func NewInMemoryEscrowRepository() *InMemoryEscrowRepository {
	return &InMemoryEscrowRepository{
		txs: make(map[string]*EscrowTransaction),
	}
}

func (r *InMemoryEscrowRepository) Save(ctx context.Context, tx *EscrowTransaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Store a copy to prevent mutation issues in concurrent operations
	txCopy := *tx
	r.txs[tx.ID] = &txCopy
	return nil
}

func (r *InMemoryEscrowRepository) Get(ctx context.Context, txID string) (*EscrowTransaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tx, exists := r.txs[txID]
	if !exists {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}
	
	txCopy := *tx
	return &txCopy, nil
}

// ============================================================================
// Default Escrow Service
// ============================================================================

// DefaultEscrowService coordinates payment gate charging, disbursement, and tracking.
type DefaultEscrowService struct {
	repo         EscrowRepository
	paymentGate  PaymentGateway
	payoutGate   DisbursementGateway
	auditLogger  *security.AuditLogger
}

var _ EscrowService = (*DefaultEscrowService)(nil)

func NewDefaultEscrowService(
	repo EscrowRepository,
	paymentGate PaymentGateway,
	payoutGate DisbursementGateway,
	auditLogger *security.AuditLogger,
) *DefaultEscrowService {
	return &DefaultEscrowService{
		repo:        repo,
		paymentGate: paymentGate,
		payoutGate:  payoutGate,
		auditLogger: auditLogger,
	}
}

func (s *DefaultEscrowService) logAudit(operatorID uint, action security.AuditAction, subjectID uint, resourceType, ip, userAgent string) {
	if s.auditLogger != nil {
		_ = s.auditLogger.Log(operatorID, action, subjectID, resourceType, ip, userAgent)
	}
}

// InitiateEscrow checks and charges the sponsor, placing the transaction in HELD status.
func (s *DefaultEscrowService) InitiateEscrow(
	ctx context.Context,
	auditCtx AuditContext,
	sponsorID, subjectID uint,
	providerID string,
	amountIn float64,
	currencyIn string,
	token string,
) (*EscrowTransaction, error) {
	if sponsorID == 0 || subjectID == 0 {
		return nil, errors.New("sponsorID and subjectID must be non-zero")
	}
	if providerID == "" {
		return nil, errors.New("providerID cannot be empty")
	}
	if amountIn <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	txID := fmt.Sprintf("esc_tx_%d", time.Now().UnixNano())
	tx := &EscrowTransaction{
		ID:         txID,
		SponsorID:  sponsorID,
		SubjectID:  subjectID,
		ProviderID: providerID,
		AmountIn:   amountIn,
		CurrencyIn: currencyIn,
		State:      EscrowStatePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Save initial pending transaction
	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save initial transaction: %w", err)
	}

	// Process the charge
	res, err := s.paymentGate.Charge(ctx, amountIn, currencyIn, token)
	if err != nil || res == nil || !res.Success {
		tx.State = EscrowStateFailed
		tx.UpdatedAt = time.Now()
		if res != nil {
			tx.ErrorMessage = res.Message
		} else if err != nil {
			tx.ErrorMessage = err.Error()
		} else {
			tx.ErrorMessage = "Unknown charge failure"
		}
		_ = s.repo.Save(ctx, tx)

		s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, subjectID, fmt.Sprintf("escrow_initiation_failed: %s", tx.ErrorMessage), auditCtx.IPAddress, auditCtx.UserAgent)

		if err != nil {
			return tx, err
		}
		return tx, fmt.Errorf("payment charge failed: %s", tx.ErrorMessage)
	}

	// Update transaction to Held
	tx.State = EscrowStateHeld
	tx.GatewayTxID = res.GatewayTxID
	tx.UpdatedAt = time.Now()
	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save held transaction: %w", err)
	}

	s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, subjectID, fmt.Sprintf("escrow_initiation_success: held tx %s", tx.ID), auditCtx.IPAddress, auditCtx.UserAgent)

	return tx, nil
}

// DisburseEscrow transfers the held funds to a local provider.
func (s *DefaultEscrowService) DisburseEscrow(
	ctx context.Context,
	auditCtx AuditContext,
	txID string,
	currencyOut string,
	exchangeRate float64,
) (*EscrowTransaction, error) {
	if txID == "" {
		return nil, errors.New("transaction ID cannot be empty")
	}
	if exchangeRate <= 0 {
		return nil, errors.New("exchange rate must be greater than zero")
	}

	tx, err := s.repo.Get(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Verify state transition
	if tx.State != EscrowStateHeld {
		return nil, fmt.Errorf("cannot disburse from state %s, must be %s", tx.State, EscrowStateHeld)
	}

	amountOut := tx.AmountIn * exchangeRate
	tx.AmountOut = amountOut
	tx.CurrencyOut = currencyOut
	tx.UpdatedAt = time.Now()

	// Call disbursement gateway
	res, err := s.payoutGate.Disburse(ctx, tx.ProviderID, amountOut, currencyOut)
	if err != nil || res == nil || !res.Success {
		tx.State = EscrowStateFailed
		if res != nil {
			tx.ErrorMessage = res.Message
		} else if err != nil {
			tx.ErrorMessage = err.Error()
		} else {
			tx.ErrorMessage = "Unknown disbursement failure"
		}
		tx.UpdatedAt = time.Now()
		_ = s.repo.Save(ctx, tx)

		s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, tx.SubjectID, fmt.Sprintf("escrow_disbursement_failed: %s", tx.ErrorMessage), auditCtx.IPAddress, auditCtx.UserAgent)

		if err != nil {
			return tx, err
		}
		return tx, fmt.Errorf("disbursement failed: %s", tx.ErrorMessage)
	}

	tx.State = EscrowStateDisbursed
	tx.DisbursementTxID = res.DisbursementTxID
	tx.ErrorMessage = ""
	tx.UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save disbursed transaction: %w", err)
	}

	s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, tx.SubjectID, fmt.Sprintf("escrow_disbursement_success: disbursed tx %s", tx.ID), auditCtx.IPAddress, auditCtx.UserAgent)

	return tx, nil
}

// RefundEscrow returns the held funds to the sponsor.
func (s *DefaultEscrowService) RefundEscrow(
	ctx context.Context,
	auditCtx AuditContext,
	txID string,
) (*EscrowTransaction, error) {
	if txID == "" {
		return nil, errors.New("transaction ID cannot be empty")
	}

	tx, err := s.repo.Get(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	if tx.State != EscrowStateHeld {
		return nil, fmt.Errorf("cannot refund from state %s, must be %s", tx.State, EscrowStateHeld)
	}

	// Call payment gateway to refund
	err = s.paymentGate.Refund(ctx, tx.GatewayTxID)
	if err != nil {
		tx.ErrorMessage = fmt.Sprintf("refund failed: %s", err.Error())
		tx.UpdatedAt = time.Now()
		_ = s.repo.Save(ctx, tx)

		s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, tx.SubjectID, fmt.Sprintf("escrow_refund_failed: %s", tx.ErrorMessage), auditCtx.IPAddress, auditCtx.UserAgent)
		return tx, err
	}

	tx.State = EscrowStateRefunded
	tx.ErrorMessage = ""
	tx.UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save refunded transaction: %w", err)
	}

	s.logAudit(auditCtx.OperatorID, security.ActionWritePHI, tx.SubjectID, fmt.Sprintf("escrow_refund_success: refunded tx %s", tx.ID), auditCtx.IPAddress, auditCtx.UserAgent)

	return tx, nil
}

// GetTransaction retrieves a transaction by ID.
func (s *DefaultEscrowService) GetTransaction(ctx context.Context, txID string) (*EscrowTransaction, error) {
	return s.repo.Get(ctx, txID)
}
