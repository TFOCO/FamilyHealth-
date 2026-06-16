package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	mock_database "github.com/fastenhealth/fasten-onprem/backend/pkg/database/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type mockFamilyUseCase struct {
	linkErr error
	links   []model.FamilyLink
	listErr error
}

func (m *mockFamilyUseCase) LinkFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint, relation, accessRole string) error {
	return m.linkErr
}

func (m *mockFamilyUseCase) CheckRelationship(ctx context.Context, operatorID uint, sponsorID, subjectID uint) (bool, error) {
	return true, nil
}

func (m *mockFamilyUseCase) ListFamilyLinks(ctx context.Context, operatorID uint, sponsorID uint) ([]model.FamilyLink, error) {
	return m.links, m.listErr
}

func (m *mockFamilyUseCase) RemoveFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint) error {
	return nil
}

func TestFamilyController_LinkFamilyMember(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success link", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		currentUser := &models.User{
			ModelBase: models.ModelBase{ID: 10},
			Username:  "operator",
		}
		mockDB.EXPECT().GetCurrentUser(gomock.Any()).Return(currentUser, nil)

		uc := &mockFamilyUseCase{}
		familyCtrl := NewFamilyController(uc, mockDB, auditLogger)

		reqBody := LinkFamilyMemberRequest{
			SponsorID:  10,
			SubjectID:  20,
			Relation:   "Father",
			AccessRole: "admin",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/family/link", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		familyCtrl.LinkFamilyMember(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
	})

	t.Run("Already exists error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		currentUser := &models.User{
			ModelBase: models.ModelBase{ID: 10},
			Username:  "operator",
		}
		mockDB.EXPECT().GetCurrentUser(gomock.Any()).Return(currentUser, nil)

		uc := &mockFamilyUseCase{
			linkErr: errors.New("family link already exists"),
		}
		familyCtrl := NewFamilyController(uc, mockDB, auditLogger)

		reqBody := LinkFamilyMemberRequest{
			SponsorID:  10,
			SubjectID:  20,
			Relation:   "Father",
			AccessRole: "admin",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/family/link", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		familyCtrl.LinkFamilyMember(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp["success"].(bool))
		assert.Equal(t, "family link already exists", resp["error"])
	})

	t.Run("Internal server error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		currentUser := &models.User{
			ModelBase: models.ModelBase{ID: 10},
			Username:  "operator",
		}
		mockDB.EXPECT().GetCurrentUser(gomock.Any()).Return(currentUser, nil)

		uc := &mockFamilyUseCase{
			linkErr: errors.New("db error"),
		}
		familyCtrl := NewFamilyController(uc, mockDB, auditLogger)

		reqBody := LinkFamilyMemberRequest{
			SponsorID:  10,
			SubjectID:  20,
			Relation:   "Father",
			AccessRole: "admin",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/family/link", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		familyCtrl.LinkFamilyMember(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestFamilyController_ListRelationships(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		currentUser := &models.User{
			ModelBase: models.ModelBase{ID: 10},
			Username:  "operator",
		}
		mockDB.EXPECT().GetCurrentUser(gomock.Any()).Return(currentUser, nil)

		expectedLinks := []model.FamilyLink{
			{
				ID:         1,
				SponsorID:  10,
				SubjectID:  20,
				Relation:   "Father",
				AccessRole: "admin",
			},
		}

		uc := &mockFamilyUseCase{
			links: expectedLinks,
		}
		familyCtrl := NewFamilyController(uc, mockDB, auditLogger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/family/links?sponsor_id=10", nil)

		familyCtrl.ListRelationships(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))

		dataList := resp["data"].([]interface{})
		assert.Len(t, dataList, 1)
		linkItem := dataList[0].(map[string]interface{})
		assert.Equal(t, float64(10), linkItem["sponsor_id"])
		assert.Equal(t, float64(20), linkItem["subject_id"])
	})

	t.Run("Invalid sponsor_id query param", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		currentUser := &models.User{
			ModelBase: models.ModelBase{ID: 10},
			Username:  "operator",
		}
		mockDB.EXPECT().GetCurrentUser(gomock.Any()).Return(currentUser, nil)

		uc := &mockFamilyUseCase{}
		familyCtrl := NewFamilyController(uc, mockDB, auditLogger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/family/links?sponsor_id=invalid", nil)

		familyCtrl.ListRelationships(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
