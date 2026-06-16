package controller

import (
	"net/http"

	"github.com/fastenhealth/fasten-onprem/backend/usecase/payments"
	"github.com/gin-gonic/gin"
)

// PaymentsController handles payment and escrow related endpoints.
type PaymentsController struct {
	paymentsUsecase *payments.Usecase
}

// NewPaymentsController creates a new PaymentsController instance.
func NewPaymentsController(paymentsUsecase *payments.Usecase) *PaymentsController {
	return &PaymentsController{
		paymentsUsecase: paymentsUsecase,
	}
}

// InitiateEscrow processes requests to initiate a payment via card or UPI.
func (ctrl *PaymentsController) InitiateEscrow(c *gin.Context) {
	var req payments.PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	resp, err := ctrl.paymentsUsecase.InitiateEscrowPayment(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate payment: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
