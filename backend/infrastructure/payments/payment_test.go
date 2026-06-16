package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*DefaultEscrowService, *InMemoryEscrowRepository, *StripeMockClient, *RazorpayMockClient, *bytes.Buffer) {
	repo := NewInMemoryEscrowRepository()
	stripeClient := NewStripeMockClient()
	razorpayClient := NewRazorpayMockClient()

	var buf bytes.Buffer
	auditLogger := security.NewAuditLogger(&buf)

	service := NewDefaultEscrowService(repo, stripeClient, razorpayClient, auditLogger)
	return service, repo, stripeClient, razorpayClient, &buf
}

func TestInitiateEscrow_Success(t *testing.T) {
	service, _, stripeClient, _, auditBuf := setupTestService(t)
	ctx := context.Background()

	auditCtx := AuditContext{
		OperatorID: 100,
		IPAddress:  "192.168.1.50",
		UserAgent:  "Mozilla/5.0",
	}

	tx, err := service.InitiateEscrow(ctx, auditCtx, 100, 200, "apollo_pharmacy", 150.00, "USD", "tok_valid")
	require.NoError(t, err)
	require.NotNil(t, tx)

	assert.Equal(t, EscrowStateHeld, tx.State)
	assert.Equal(t, 150.00, tx.AmountIn)
	assert.Equal(t, "USD", tx.CurrencyIn)
	assert.NotEmpty(t, tx.GatewayTxID)
	assert.Empty(t, tx.ErrorMessage)

	// Verify mock client recorded the charge
	assert.Equal(t, 150.00, stripeClient.ChargedAmount)
	assert.Equal(t, 1, stripeClient.ChargedCount)

	// Verify Audit Log was written
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_initiation_success")
	assert.Contains(t, logStr, "192.168.1.50")
	assert.Contains(t, logStr, "Mozilla/5.0")

	// Parse JSON log line to verify structured output
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	require.NoError(t, err)
	assert.Equal(t, uint(100), record.OperatorID)
	assert.Equal(t, uint(200), record.TargetPatientID)
	assert.Equal(t, security.ActionWritePHI, record.Action)
}

func TestInitiateEscrow_ChargeDeclined(t *testing.T) {
	service, _, stripeClient, _, auditBuf := setupTestService(t)
	ctx := context.Background()

	auditCtx := AuditContext{
		OperatorID: 100,
		IPAddress:  "192.168.1.50",
		UserAgent:  "Mozilla/5.0",
	}

	tx, err := service.InitiateEscrow(ctx, auditCtx, 100, 200, "apollo_pharmacy", 150.00, "USD", "tok_decline")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payment charge failed")
	require.NotNil(t, tx)

	assert.Equal(t, EscrowStateFailed, tx.State)
	assert.Equal(t, "Card declined by issuer", tx.ErrorMessage)

	// Verify mock client didn't record successful charge
	assert.Equal(t, 0.0, stripeClient.ChargedAmount)
	assert.Equal(t, 0, stripeClient.ChargedCount)

	// Verify Audit Log recorded the failure
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_initiation_failed")
	assert.Contains(t, logStr, "Card declined by issuer")
}

func TestInitiateEscrow_GatewayError(t *testing.T) {
	service, _, stripeClient, _, _ := setupTestService(t)
	ctx := context.Background()

	stripeClient.SimulateError = errors.New("network timeout")

	auditCtx := AuditContext{OperatorID: 100}

	tx, err := service.InitiateEscrow(ctx, auditCtx, 100, 200, "apollo_pharmacy", 150.00, "USD", "tok_valid")
	require.Error(t, err)
	assert.Equal(t, "network timeout", err.Error())
	require.NotNil(t, tx)
	assert.Equal(t, EscrowStateFailed, tx.State)
	assert.Equal(t, "network timeout", tx.ErrorMessage)
}

func TestDisburseEscrow_Success(t *testing.T) {
	service, repo, _, razorpayClient, auditBuf := setupTestService(t)
	ctx := context.Background()

	auditCtx := AuditContext{
		OperatorID: 100,
		IPAddress:  "192.168.1.50",
		UserAgent:  "Mozilla/5.0",
	}

	// 1. Setup pre-existing HELD escrow transaction
	heldTx := &EscrowTransaction{
		ID:          "esc_tx_123",
		SponsorID:   100,
		SubjectID:   200,
		ProviderID:  "apollo_pharmacy",
		AmountIn:    150.00,
		CurrencyIn:  "USD",
		State:       EscrowStateHeld,
		GatewayTxID: "ch_stripe_123",
	}
	require.NoError(t, repo.Save(ctx, heldTx))

	// 2. Disburse
	disbursedTx, err := service.DisburseEscrow(ctx, auditCtx, "esc_tx_123", "INR", 83.5)
	require.NoError(t, err)
	require.NotNil(t, disbursedTx)

	assert.Equal(t, EscrowStateDisbursed, disbursedTx.State)
	assert.Equal(t, 12525.0, disbursedTx.AmountOut) // 150 * 83.5
	assert.Equal(t, "INR", disbursedTx.CurrencyOut)
	assert.NotEmpty(t, disbursedTx.DisbursementTxID)
	assert.Empty(t, disbursedTx.ErrorMessage)

	// Verify Razorpay Client was called
	assert.Equal(t, 12525.0, razorpayClient.DisbursedAmount)
	assert.Equal(t, 1, razorpayClient.DisbursedCount)

	// Verify Audit Log
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_disbursement_success")
	assert.Contains(t, logStr, "disbursed tx esc_tx_123")
}

func TestDisburseEscrow_Failure(t *testing.T) {
	service, repo, _, razorpayClient, auditBuf := setupTestService(t)
	ctx := context.Background()

	auditCtx := AuditContext{OperatorID: 100}

	// 1. Setup pre-existing HELD escrow transaction
	heldTx := &EscrowTransaction{
		ID:          "esc_tx_123",
		SponsorID:   100,
		SubjectID:   200,
		ProviderID:  "fail_provider",
		AmountIn:    150.00,
		CurrencyIn:  "USD",
		State:       EscrowStateHeld,
		GatewayTxID: "ch_stripe_123",
	}
	require.NoError(t, repo.Save(ctx, heldTx))

	// 2. Disburse which will fail due to provider ID "fail_provider"
	disbursedTx, err := service.DisburseEscrow(ctx, auditCtx, "esc_tx_123", "INR", 83.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disbursement failed")
	require.NotNil(t, disbursedTx)

	assert.Equal(t, EscrowStateFailed, disbursedTx.State)
	assert.Equal(t, "Payout failed: routing error", disbursedTx.ErrorMessage)

	// Verify Razorpay Client registered the attempt but failed
	assert.Equal(t, 0.0, razorpayClient.DisbursedAmount)
	assert.Equal(t, 0, razorpayClient.DisbursedCount)

	// Verify Audit Log recorded failure
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_disbursement_failed")
}

func TestDisburseEscrow_InvalidState(t *testing.T) {
	service, repo, _, _, _ := setupTestService(t)
	ctx := context.Background()
	auditCtx := AuditContext{OperatorID: 100}

	// 1. Create a transaction that is PENDING, not HELD
	pendingTx := &EscrowTransaction{
		ID:         "esc_tx_999",
		SponsorID:  100,
		SubjectID:  200,
		ProviderID: "apollo_pharmacy",
		AmountIn:   150.00,
		CurrencyIn: "USD",
		State:      EscrowStatePending,
	}
	require.NoError(t, repo.Save(ctx, pendingTx))

	// 2. Attempt disburse
	_, err := service.DisburseEscrow(ctx, auditCtx, "esc_tx_999", "INR", 83.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot disburse from state PENDING, must be HELD")
}

func TestRefundEscrow_Success(t *testing.T) {
	service, repo, stripeClient, _, auditBuf := setupTestService(t)
	ctx := context.Background()

	auditCtx := AuditContext{
		OperatorID: 100,
		IPAddress:  "192.168.1.50",
		UserAgent:  "Mozilla/5.0",
	}

	// 1. Setup pre-existing HELD escrow transaction
	heldTx := &EscrowTransaction{
		ID:          "esc_tx_456",
		SponsorID:   100,
		SubjectID:   200,
		ProviderID:  "apollo_pharmacy",
		AmountIn:    100.00,
		CurrencyIn:  "USD",
		State:       EscrowStateHeld,
		GatewayTxID: "ch_stripe_456",
	}
	require.NoError(t, repo.Save(ctx, heldTx))

	// 2. Refund
	refundedTx, err := service.RefundEscrow(ctx, auditCtx, "esc_tx_456")
	require.NoError(t, err)
	require.NotNil(t, refundedTx)

	assert.Equal(t, EscrowStateRefunded, refundedTx.State)
	assert.Empty(t, refundedTx.ErrorMessage)

	// Verify Stripe Refund call
	assert.Equal(t, 1, stripeClient.RefundedCount)

	// Verify Audit Log
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_refund_success")
	assert.Contains(t, logStr, "refunded tx esc_tx_456")
}

func TestRefundEscrow_Failure(t *testing.T) {
	service, repo, stripeClient, _, auditBuf := setupTestService(t)
	ctx := context.Background()

	stripeClient.SimulateError = errors.New("charge already refunded or disputed")
	auditCtx := AuditContext{OperatorID: 100}

	// 1. Setup pre-existing HELD escrow transaction
	heldTx := &EscrowTransaction{
		ID:          "esc_tx_456",
		SponsorID:   100,
		SubjectID:   200,
		ProviderID:  "apollo_pharmacy",
		AmountIn:    100.00,
		CurrencyIn:  "USD",
		State:       EscrowStateHeld,
		GatewayTxID: "ch_stripe_456",
	}
	require.NoError(t, repo.Save(ctx, heldTx))

	// 2. Refund which should fail
	tx, err := service.RefundEscrow(ctx, auditCtx, "esc_tx_456")
	require.Error(t, err)
	assert.Equal(t, "charge already refunded or disputed", err.Error())
	assert.Equal(t, EscrowStateHeld, tx.State) // Remains held if refund fails
	assert.Equal(t, "refund failed: charge already refunded or disputed", tx.ErrorMessage)

	// Verify Audit Log
	logStr := auditBuf.String()
	assert.Contains(t, logStr, "escrow_refund_failed")
}

func TestRefundEscrow_InvalidState(t *testing.T) {
	service, repo, _, _, _ := setupTestService(t)
	ctx := context.Background()
	auditCtx := AuditContext{OperatorID: 100}

	// 1. Create a transaction that is already DISBURSED
	disbursedTx := &EscrowTransaction{
		ID:         "esc_tx_888",
		SponsorID:  100,
		SubjectID:  200,
		ProviderID: "apollo_pharmacy",
		AmountIn:   150.00,
		CurrencyIn: "USD",
		State:      EscrowStateDisbursed,
	}
	require.NoError(t, repo.Save(ctx, disbursedTx))

	// 2. Attempt refund
	_, err := service.RefundEscrow(ctx, auditCtx, "esc_tx_888")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot refund from state DISBURSED, must be HELD")
}

func TestRazorpay_GenerateUPIIntent(t *testing.T) {
	client := NewRazorpayMockClient()
	ctx := context.Background()

	uri, err := client.GenerateUPIIntent(ctx, 450.50, "familyhealth@oksbi", "FamilyHealth+", "tx_ref_123", "Care Escrow Payout")
	require.NoError(t, err)
	assert.NotEmpty(t, uri)

	// Validate NPIC standard format parameters
	assert.Contains(t, uri, "upi://pay?")
	assert.Contains(t, uri, "pa=familyhealth@oksbi")
	assert.Contains(t, uri, "pn=FamilyHealth+")
	assert.Contains(t, uri, "tr=tx_ref_123")
	assert.Contains(t, uri, "tn=Care Escrow Payout")
	assert.Contains(t, uri, "am=450.50")
	assert.Contains(t, uri, "cu=INR")
}
