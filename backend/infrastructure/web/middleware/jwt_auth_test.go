package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/auth"
	mock_config "github.com/fastenhealth/fasten-onprem/backend/pkg/config/mock"
	mock_database "github.com/fastenhealth/fasten-onprem/backend/pkg/database/mock"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJWTAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success authentication", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)

		encryptionKey := "6368616e676570617373776f72646d7573746265333262797465736b65792121"
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return(encryptionKey).AnyTimes()

		// Generate a valid token
		userUUID := uuid.New()
		userData := models.User{
			ModelBase: models.ModelBase{
				ID:        userUUID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			FullName: "John Doe",
			Username: "john.doe",
			Email:    "john@example.com",
		}

		token, err := auth.JwtGenerateFastenTokenFromUser(userData, encryptionKey)
		assert.NoError(t, err)

		mockDB.EXPECT().GetUserByUsername(gomock.Any(), "john.doe").Return(&userData, nil)

		r := gin.New()
		r.Use(JWTAuth(mockDB, mockConfig))
		r.GET("/protected", func(c *gin.Context) {
			opID, exists := c.Get("operator_id")
			assert.True(t, exists)
			assert.Equal(t, UUIDToUint(userUUID), opID.(uint))
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)

		r := gin.New()
		r.Use(JWTAuth(mockDB, mockConfig))
		r.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authorization header is required")
	})

	t.Run("invalid token format", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)

		r := gin.New()
		r.Use(JWTAuth(mockDB, mockConfig))
		r.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "BearerInvalidFormat tokenhere")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDB := mock_database.NewMockDatabaseRepository(ctrl)
		mockConfig := mock_config.NewMockInterface(ctrl)

		encryptionKey := "6368616e676570617373776f72646d7573746265333262797465736b65792121"
		mockConfig.EXPECT().GetString("jwt.issuer.key").Return(encryptionKey).AnyTimes()

		userData := models.User{
			ModelBase: models.ModelBase{ID: uuid.New()},
			FullName:  "Ghost User",
			Username:  "ghost",
		}

		token, err := auth.JwtGenerateFastenTokenFromUser(userData, encryptionKey)
		assert.NoError(t, err)

		mockDB.EXPECT().GetUserByUsername(gomock.Any(), "ghost").Return(nil, errors.New("not found"))

		r := gin.New()
		r.Use(JWTAuth(mockDB, mockConfig))
		r.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "user not found")
	})
}
