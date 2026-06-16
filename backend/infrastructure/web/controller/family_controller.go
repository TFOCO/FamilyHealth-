package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/database"
	"github.com/fastenhealth/fasten-onprem/backend/usecase/family"
	"github.com/gin-gonic/gin"
)

type FamilyController struct {
	familyUseCase family.FamilyUseCase
	dbRepo        database.DatabaseRepository
	auditLogger   *security.AuditLogger
}

func NewFamilyController(familyUseCase family.FamilyUseCase, dbRepo database.DatabaseRepository, auditLogger *security.AuditLogger) *FamilyController {
	return &FamilyController{
		familyUseCase: familyUseCase,
		dbRepo:        dbRepo,
		auditLogger:   auditLogger,
	}
}

type LinkFamilyMemberRequest struct {
	SponsorID  uint   `json:"sponsor_id" binding:"required"`
	SubjectID  uint   `json:"subject_id" binding:"required"`
	Relation   string `json:"relation" binding:"required"`
	AccessRole string `json:"access_role" binding:"required"`
}

func (ctrl *FamilyController) LinkFamilyMember(c *gin.Context) {
	var req LinkFamilyMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	currentUser, err := ctrl.dbRepo.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	err = ctrl.familyUseCase.LinkFamilyMember(ctx, currentUser.ID, req.SponsorID, req.SubjectID, req.Relation, req.AccessRole)
	if err != nil {
		if err.Error() == "family link already exists" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (ctrl *FamilyController) ListRelationships(c *gin.Context) {
	ctx := c.Request.Context()
	currentUser, err := ctrl.dbRepo.GetCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	sponsorID := currentUser.ID
	sponsorIDStr := c.Query("sponsor_id")
	if sponsorIDStr != "" {
		id, err := strconv.ParseUint(sponsorIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid sponsor_id parameter"})
			return
		}
		sponsorID = uint(id)
	}

	links, err := ctrl.familyUseCase.ListFamilyLinks(ctx, currentUser.ID, sponsorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": links})
}
