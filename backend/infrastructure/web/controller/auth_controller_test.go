package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	mock_database "github.com/fastenhealth/fasten-onprem/backend/pkg/database/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAuthController_Signup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success first user signup - becomes admin", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		mockDB.EXPECT().GetUserCount(gomock.Any()).Return(0, nil)
		mockDB.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Do(func(_ interface{}, user *models.User) {
			assert.Equal(t, pkg.UserRoleAdmin, user.Role)
			assert.Equal(t, "adminuser", user.Username)
		}).Return(nil)
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return("6368616e676570617373776f72646d7573746265333262797465736b65792121").AnyTimes()

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SignupRequest{
			Username: "adminuser",
			Password: "securepassword",
			FullName: "Admin User",
			Email:    "admin@example.com",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signup(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
		assert.NotEmpty(t, resp["token"])
	})

	t.Run("Success subsequent user signup - becomes user", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		mockDB.EXPECT().GetUserCount(gomock.Any()).Return(5, nil)
		mockDB.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Do(func(_ interface{}, user *models.User) {
			assert.Equal(t, pkg.UserRoleUser, user.Role)
			assert.Equal(t, "regularuser", user.Username)
		}).Return(nil)
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return("6368616e676570617373776f72646d7573746265333262797465736b65792121").AnyTimes()

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SignupRequest{
			Username: "regularuser",
			Password: "securepassword",
			FullName: "Regular User",
			Email:    "user@example.com",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signup(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
		assert.NotEmpty(t, resp["token"])
	})

	t.Run("Bad Request - Missing Fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SignupRequest{
			Username: "",
			Password: "securepassword",
			FullName: "Regular User",
			Email:    "",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signup(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.False(t, resp["success"].(bool))
		assert.Contains(t, resp["error"], "required fields")
	})

	t.Run("Database Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		mockDB.EXPECT().GetUserCount(gomock.Any()).Return(0, errors.New("db error"))

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SignupRequest{
			Username: "user",
			Password: "password",
			FullName: "User",
			Email:    "user@example.com",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signup", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signup(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAuthController_Signin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success Signin", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		hashedUser := &models.User{
			FullName: "Regular User",
			Username: "regularuser",
			Email:    "user@example.com",
			Role:     pkg.UserRoleUser,
		}
		err := hashedUser.HashPassword("securepassword")
		assert.NoError(t, err)

		mockDB.EXPECT().GetUserByUsername(gomock.Any(), "regularuser").Return(hashedUser, nil)
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return("6368616e676570617373776f72646d7573746265333262797465736b65792121").AnyTimes()

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SigninRequest{
			Username: "regularuser",
			Password: "securepassword",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signin", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signin(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp["success"].(bool))
		assert.NotEmpty(t, resp["token"])
	})

	t.Run("Unauthorized - Invalid Password", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		hashedUser := &models.User{
			FullName: "Regular User",
			Username: "regularuser",
			Email:    "user@example.com",
			Role:     pkg.UserRoleUser,
		}
		err := hashedUser.HashPassword("securepassword")
		assert.NoError(t, err)

		mockDB.EXPECT().GetUserByUsername(gomock.Any(), "regularuser").Return(hashedUser, nil)

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SigninRequest{
			Username: "regularuser",
			Password: "wrongpassword",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signin", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signin(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, logBuf.String(), "AUTH_FAIL")
	})

	t.Run("Unauthorized - User Not Found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)
		var logBuf bytes.Buffer
		auditLogger := security.NewAuditLogger(&logBuf)

		mockDB.EXPECT().GetUserByUsername(gomock.Any(), "nonexistent").Return(nil, errors.New("not found"))

		authCtrl := NewAuthController(mockDB, mockConfig, auditLogger)

		reqBody := SigninRequest{
			Username: "nonexistent",
			Password: "password",
		}
		jsonData, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/signin", bytes.NewReader(jsonData))
		c.Request.Header.Set("Content-Type", "application/json")

		authCtrl.Signin(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, logBuf.String(), "AUTH_FAIL")
	})
}
