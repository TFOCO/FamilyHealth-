package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/auth"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	dbRepo      database.DatabaseRepository
	appConfig   config.Interface
	auditLogger *security.AuditLogger
}

func NewAuthController(dbRepo database.DatabaseRepository, appConfig config.Interface, auditLogger *security.AuditLogger) *AuthController {
	return &AuthController{
		dbRepo:      dbRepo,
		appConfig:   appConfig,
		auditLogger: auditLogger,
	}
}

type SignupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required"`
}

func (ctrl *AuthController) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" ||
		strings.TrimSpace(req.FullName) == "" || strings.TrimSpace(req.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "username, password, full_name, and email are required fields"})
		return
	}

	ctx := c.Request.Context()
	userCount, err := ctrl.dbRepo.GetUserCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to query user database"})
		return
	}

	role := pkg.UserRoleUser
	if userCount == 0 {
		role = pkg.UserRoleAdmin
	}

	newUser := &models.User{
		Username: req.Username,
		Password: req.Password, // GORM CreateUser will hash this
		FullName: req.FullName,
		Email:    req.Email,
		Role:     role,
	}

	if err := ctrl.dbRepo.CreateUser(ctx, newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	signingKey := ctrl.appConfig.GetString("jwt.issuer.key")
	if signingKey == "" {
		signingKey = "default_secret_key_if_none_configured"
	}

	token, err := auth.JwtGenerateFastenTokenFromUser(*newUser, signingKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"id":        newUser.ID,
			"username":  newUser.Username,
			"full_name": newUser.FullName,
			"email":     newUser.Email,
			"role":      newUser.Role,
		},
	})
}

type SigninRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (ctrl *AuthController) Signin(c *gin.Context) {
	var req SigninRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "username and password are required"})
		return
	}

	ctx := c.Request.Context()
	foundUser, err := ctrl.dbRepo.GetUserByUsername(ctx, req.Username)
	if err != nil || foundUser == nil {
		_ = ctrl.auditLogger.Log(
			0,
			security.ActionAuthFail,
			0,
			"Auth/Signin",
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid username or password"})
		return
	}

	if err := foundUser.CheckPassword(req.Password); err != nil {
		_ = ctrl.auditLogger.Log(
			foundUser.ID,
			security.ActionAuthFail,
			0,
			"Auth/Signin",
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid username or password"})
		return
	}

	signingKey := ctrl.appConfig.GetString("jwt.issuer.key")
	if signingKey == "" {
		signingKey = "default_secret_key_if_none_configured"
	}

	token, err := auth.JwtGenerateFastenTokenFromUser(*foundUser, signingKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"id":        foundUser.ID,
			"username":  foundUser.Username,
			"full_name": foundUser.FullName,
			"email":     foundUser.Email,
			"role":      foundUser.Role,
		},
	})
}
