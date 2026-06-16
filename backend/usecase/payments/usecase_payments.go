package payments

import (
	"context"
	"errors"
	"fmt"
)

// PaymentRequest represents the input to initiate a payment.
type PaymentRequest struct {
	SponsorID uint    `json:"sponsor_id"`
	SubjectID uint    `json:"subject_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"` // "USD", "EUR", "INR"
	Method    string  `json:"method"`   // "UPI", "CARD"
}

// PaymentResponse represents the response containing payment intents.
type PaymentResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"` // "PENDING", "COMPLETED", "FAILED"
	ClientSecret  string `json:"client_secret,omitempty"`
	UPILink       string `json:"upi_link,omitempty"`
}

// Usecase handles payment logic.
type Usecase struct {
	// In a real application, you would inject repositories and payment gateway clients here.
}

// NewUsecase creates a new payment usecase.
func NewUsecase() *Usecase {
	return &Usecase{}
}

// InitiateEscrowPayment starts the payment process.
func (u *Usecase) InitiateEscrowPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	// Mocking payment gateway logic
	transactionID := fmt.Sprintf("txn_%d", req.SponsorID)

	response := &PaymentResponse{
		TransactionID: transactionID,
		Status:        "PENDING",
	}

	if req.Method == "UPI" {
		if req.Currency != "INR" {
			return nil, errors.New("UPI requires INR currency")
		}
		// Mock UPI intent link
		response.UPILink = fmt.Sprintf("upi://pay?pa=merchant@upi&pn=FamilyHealth&am=%.2f&cu=INR&tr=%s", req.Amount, transactionID)
	} else if req.Method == "CARD" {
		// Mock Stripe Client Secret
		response.ClientSecret = fmt.Sprintf("pi_%s_secret_mock", transactionID)
	} else {
		return nil, errors.New("unsupported payment method")
	}

	return response, nil
}
