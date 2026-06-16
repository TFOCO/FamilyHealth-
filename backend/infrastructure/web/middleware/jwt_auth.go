package middleware

import (
	"hash/fnv"
	"net/http"
	"strings"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/auth"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// JWTAuth validates incoming requests using Bearer JWT tokens and sets the operator ID context.
func JWTAuth(dbRepo database.DatabaseRepository, appConfig config.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header format must be Bearer <token>"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		signingKey := appConfig.GetString("jwt.issuer.key")
		if signingKey == "" {
			signingKey = "default_secret_key_if_none_configured"
		}

		claims, err := auth.JwtValidateFastenToken(signingKey, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token: " + err.Error()})
			c.Abort()
			return
		}

		// Look up user by username to get GORM UUID
		user, err := dbRepo.GetUserByUsername(c.Request.Context(), claims.Subject)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}

		// Convert UUID to uint deterministically for family_links mapping compatibility
		operatorID := UUIDToUint(user.ID)

		// Set operator context fields in the Gin Context
		c.Set("operator_id", operatorID)
		c.Set("username", user.Username)

		c.Next()
	}
}

// UUIDToUint converts a UUID into a deterministic uint value using FNV hashing.
func UUIDToUint(uid uuid.UUID) uint {
	h := fnv.New32a()
	_, _ = h.Write(uid[:])
	return uint(h.Sum32())
}
