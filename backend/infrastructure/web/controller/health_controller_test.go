package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/persistence"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, *persistence.GormRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	hexKey := "6368616e676570617373776f72646d7573746265333262797465736b65792121"
	crypto, err := security.NewCryptoEngine(hexKey)
	require.NoError(t, err)

	repo := persistence.NewGormRepository(db, crypto)
	err = repo.Migrate()
	require.NoError(t, err)

	return db, repo
}

func TestHealthController_RecordTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, repo := setupTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewHealthController(repo, auditLogger)

	r := gin.New()
	r.POST("/telemetry", func(c *gin.Context) {
		c.Set("operator_id", uint(12)) // mock operator id
		ctrl.RecordTelemetry(c)
	})

	t.Run("success", func(t *testing.T) {
		reqPayload := TelemetryRequest{
			SubjectID:   99,
			VitalType:   "blood_pressure",
			ValueMetric: 120.5,
			ValueUnit:   "mmHg",
			ContextData: `{"diastolic":80}`,
			RecordedAt:  time.Now(),
		}
		jsonData, _ := json.Marshal(reqPayload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewReader(jsonData))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp model.VitalTelemetry
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, uint(99), resp.SubjectID)
		assert.Equal(t, "blood_pressure", resp.VitalType)
		assert.Equal(t, 120.5, resp.ValueMetric)

		// Verify audit log write PHI
		assert.Contains(t, logBuf.String(), "WRITE_PHI")
		assert.Contains(t, logBuf.String(), "VitalTelemetry")
	})

	t.Run("missing subject_id", func(t *testing.T) {
		reqPayload := TelemetryRequest{
			VitalType:   "blood_pressure",
			ValueMetric: 120.5,
			ValueUnit:   "mmHg",
		}
		jsonData, _ := json.Marshal(reqPayload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewReader(jsonData))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHealthController_ListTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, repo := setupTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewHealthController(repo, auditLogger)

	r := gin.New()
	r.GET("/telemetry", func(c *gin.Context) {
		c.Set("operator_id", uint(12))
		ctrl.ListTelemetry(c)
	})

	// Seed some telemetry
	v1 := &model.VitalTelemetry{
		SubjectID:   88,
		VitalType:   "blood_pressure",
		ValueMetric: 130,
		ValueUnit:   "mmHg",
		RecordedAt:  time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, repo.RecordVitals(context.Background(), v1))

	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/telemetry?subject_id=88", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp []model.VitalTelemetry
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, uint(88), resp[0].SubjectID)

		// Verify audit log read PHI
		assert.Contains(t, logBuf.String(), "READ_PHI")
	})

	t.Run("missing subject_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/telemetry", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHealthController_GetSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, repo := setupTestDB(t)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewHealthController(repo, auditLogger)

	// Seed high blood pressure
	v1 := &model.VitalTelemetry{
		SubjectID:   77,
		VitalType:   "blood_pressure",
		ValueMetric: 145.0,
		ValueUnit:   "mmHg",
		ContextData: `{"diastolic":90}`,
		RecordedAt:  time.Now(),
	}
	require.NoError(t, repo.RecordVitals(context.Background(), v1))

	r := gin.New()
	r.POST("/summary", func(c *gin.Context) {
		c.Set("operator_id", uint(12))
		ctrl.GetSummary(c)
	})
	r.GET("/summary", func(c *gin.Context) {
		c.Set("operator_id", uint(12))
		ctrl.GetSummary(c)
	})

	t.Run("post success english", func(t *testing.T) {
		reqPayload := SummaryRequest{
			SubjectID:         77,
			PreferredLanguage: "English",
		}
		jsonData, _ := json.Marshal(reqPayload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/summary", bytes.NewReader(jsonData))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp["status"])
		assert.Contains(t, resp["summary"], "blood pressure is elevated")
		assert.Contains(t, resp["check_in_script"], "Did you take your BP pill today?")
	})

	t.Run("get success hindi", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/summary?subject_id=77&preferred_language=Hindi", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp["status"])
		assert.Contains(t, resp["summary"], "रक्तचाप")
	})

	t.Run("get success portuguese", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/summary?subject_id=77&preferred_language=Portuguese", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "success", resp["status"])
		assert.Contains(t, resp["summary"], "elevada")
	})
}

func TestHealthController_GetDrugEquivalent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var logBuf bytes.Buffer
	auditLogger := security.NewAuditLogger(&logBuf)

	ctrl := NewHealthController(nil, auditLogger)

	r := gin.New()
	r.GET("/drug-equivalent", func(c *gin.Context) {
		c.Set("operator_id", uint(12))
		ctrl.GetDrugEquivalent(c)
	})

	t.Run("glycomet india success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/drug-equivalent?brand_name=glycomet&region=India", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DrugEquivalence
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Metformin", resp.GenericChemical)
		assert.Contains(t, resp.InternationalEquivalents.USA, "Glucophage")
	})

	t.Run("nolotil portugal success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/drug-equivalent?brand=nolotil&country=Portugal", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DrugEquivalence
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Contains(t, resp.GenericChemical, "Metamizole")
		assert.Contains(t, resp.InternationalEquivalents.Europe, "Nolotil")
	})

	t.Run("unknown drug success fallback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/drug-equivalent?brand=mysteriouspill&country=India", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DrugEquivalence
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Contains(t, resp.GenericChemical, "Mysteriouspill")
		assert.Contains(t, resp.InternationalEquivalents.USA, "Mysteriouspill Equivalent US")
	})

	t.Run("missing params", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/drug-equivalent?brand=mysteriouspill", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
