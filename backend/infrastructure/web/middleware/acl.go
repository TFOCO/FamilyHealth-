package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// ACLMiddleware handles access authorization for accessing PHI records.
type ACLMiddleware struct {
	familyRepo  repository.FamilyRepository
	auditLogger *security.AuditLogger
}

// NewACLMiddleware creates a new ACL middleware instance.
func NewACLMiddleware(repo repository.FamilyRepository, logger *security.AuditLogger) *ACLMiddleware {
	return &ACLMiddleware{
		familyRepo:  repo,
		auditLogger: logger,
	}
}

// AuthorizePHIAccess checks if the current operator has access to the target subject's health records.
func (m *ACLMiddleware) AuthorizePHIAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve logged-in operator ID from context (injected by Auth middleware)
		operatorVal, exists := c.Get("operator_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated request"})
			c.Abort()
			return
		}
		operatorID, ok := operatorVal.(uint)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid operator identity"})
			c.Abort()
			return
		}

		// Retrieve target patient (subject_id) from URL path or query parameter
		subjectStr := c.Param("subject_id")
		if subjectStr == "" {
			subjectStr = c.Query("subject_id")
		}

		if subjectStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing target patient context (subject_id)"})
			c.Abort()
			return
		}

		subjectID64, err := strconv.ParseUint(subjectStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target patient identity format"})
			c.Abort()
			return
		}
		subjectID := uint(subjectID64)

		// Bypass checks if operator is accessing their own records
		if operatorID == subjectID {
			c.Next()
			return
		}

		// Query relationship from GORM persistence repository
		link, err := m.familyRepo.GetLink(c.Request.Context(), operatorID, subjectID)
		if err != nil || link == nil {
			// HIPAA Violation Audit Entry
			m.auditLogger.Log(
				operatorID,
				security.ActionAuthFail,
				subjectID,
				"Observation/DiagnosticReport",
				c.ClientIP(),
				c.Request.UserAgent(),
			)

			c.JSON(http.StatusForbidden, gin.H{"error": "access denied: you do not have permission to view this patient's records"})
			c.Abort()
			return
		}

		// Proceed to controller
		c.Next()
	}
}
