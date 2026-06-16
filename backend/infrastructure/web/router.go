package web

import (
	"net/http"

	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/web/controller"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/web/middleware"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the Gin engine with CORS, routes, controllers, and middlewares.
func SetupRouter(
	dbRepo database.DatabaseRepository,
	telemetryRepo repository.TelemetryRepository,
	familyRepo repository.FamilyRepository,
	emergencyRepo repository.EmergencyRepository,
	appConfig config.Interface,
	auditLogger *security.AuditLogger,
) *gin.Engine {
	r := gin.New()

	// Configure standard Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())

	// Initialize Controllers
	healthCtrl := controller.NewHealthController(telemetryRepo, auditLogger)
	doctorPortalCtrl := controller.NewDoctorPortalController(emergencyRepo, telemetryRepo, auditLogger)
	
	// Initialize Usecases
	paymentsUsecase := payments.NewUsecase()
	paymentsCtrl := controller.NewPaymentsController(paymentsUsecase)

	// Initialize Middlewares
	aclMiddleware := middleware.NewACLMiddleware(familyRepo, auditLogger)

	// Root API Group
	v1 := r.Group("/api/v1")

	// 1. Unauthenticated endpoints
	// Paramedics / emergency responders open access to resolve emergency QR code profiles
	v1.GET("/emergency/qr/:hash", doctorPortalCtrl.ResolveAccessCode)

	// 2. JWT Authenticated Health/Vitals group
	health := v1.Group("/health")
	health.Use(middleware.JWTAuth(dbRepo, appConfig))
	{
		// Drug equivalence lookup (does not require patient-specific ACL check)
		health.GET("/drug-equivalent", healthCtrl.GetDrugEquivalent)
		health.GET("/drug-equivalence", healthCtrl.GetDrugEquivalent)

		// Patient-specific health records endpoints (require HIPAA ACL verification)
		healthWithACL := health.Group("")
		healthWithACL.Use(aclMiddleware.AuthorizePHIAccess())
		{
			healthWithACL.POST("/telemetry", healthCtrl.RecordTelemetry)
			healthWithACL.GET("/telemetry", healthCtrl.ListTelemetry)
			healthWithACL.POST("/summary", healthCtrl.GetSummary)
			healthWithACL.GET("/summary", healthCtrl.GetSummary)
		}
	}

	// 3. Payments endpoints
	paymentsGroup := v1.Group("/payments")
	paymentsGroup.Use(middleware.JWTAuth(dbRepo, appConfig))
	{
		paymentsGroup.POST("/escrow/initiate", paymentsCtrl.InitiateEscrow)
	}

	return r
}

// CORSMiddleware provides a robust Cross-Origin Resource Sharing configuration for the API gateway.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
