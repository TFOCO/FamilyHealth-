package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

type mockTelemetryRepository struct {
	vitals    []model.VitalTelemetry
	recordErr error
	listErr   error
}

func newMockTelemetryRepository() *mockTelemetryRepository {
	return &mockTelemetryRepository{
		vitals: make([]model.VitalTelemetry, 0),
	}
}

func (m *mockTelemetryRepository) RecordVitals(ctx context.Context, vitals *model.VitalTelemetry) error {
	if m.recordErr != nil {
		return m.recordErr
	}
	m.vitals = append(m.vitals, *vitals)
	return nil
}

func (m *mockTelemetryRepository) ListVitals(ctx context.Context, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var res []model.VitalTelemetry
	for _, v := range m.vitals {
		if v.SubjectID == subjectID && (vitalType == "" || v.VitalType == vitalType) {
			res = append(res, v)
		}
	}
	if limit > 0 && len(res) > limit {
		res = res[len(res)-limit:]
	}
	return res, nil
}

func TestProcessTelemetry(t *testing.T) {
	ctx := context.Background()
	repo := newMockTelemetryRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	llm := NewMockLLMClient()
	uc := NewHealthUseCase(repo, llm, logger)

	operatorID := uint(10)
	telemetry := &model.VitalTelemetry{
		SubjectID:   uint(2),
		VitalType:   "blood_pressure",
		ValueMetric: 120.5,
		ValueUnit:   "mmHg",
		ContextData: `{"systolic": 120.5, "diastolic": 80.0}`,
		RecordedAt:  time.Now(),
	}

	// 1. Success case
	err := uc.ProcessTelemetry(ctx, operatorID, telemetry)
	assert.NoError(t, err)
	assert.Len(t, repo.vitals, 1)
	assert.Equal(t, 120.5, repo.vitals[0].ValueMetric)

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionWritePHI, record.Action)
	assert.Equal(t, telemetry.SubjectID, record.TargetPatientID)
	assert.Equal(t, "VitalTelemetry", record.ResourceType)

	// 2. Error case
	repo.recordErr = errors.New("write failure")
	err = uc.ProcessTelemetry(ctx, operatorID, telemetry)
	assert.Error(t, err)
}

func TestListTelemetry(t *testing.T) {
	ctx := context.Background()
	repo := newMockTelemetryRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	llm := NewMockLLMClient()
	uc := NewHealthUseCase(repo, llm, logger)

	operatorID := uint(10)
	subjectID := uint(2)

	_ = repo.RecordVitals(ctx, &model.VitalTelemetry{SubjectID: subjectID, VitalType: "blood_pressure", ValueMetric: 120.0})
	_ = repo.RecordVitals(ctx, &model.VitalTelemetry{SubjectID: subjectID, VitalType: "blood_pressure", ValueMetric: 130.0})

	auditBuf.Reset()
	vitals, err := uc.ListTelemetry(ctx, operatorID, subjectID, "blood_pressure", 10)
	assert.NoError(t, err)
	assert.Len(t, vitals, 2)

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionReadPHI, record.Action)
	assert.Equal(t, subjectID, record.TargetPatientID)
}

func TestGeneratePatientSummary(t *testing.T) {
	ctx := context.Background()
	repo := newMockTelemetryRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	llm := NewMockLLMClient()
	uc := NewHealthUseCase(repo, llm, logger)

	operatorID := uint(10)
	subjectID := uint(2)

	// 1. Check Diabetes summary triggers (English default)
	medicalJSON := `{"medications": [{"name": "Metformin"}], "diagnosis": "diabetic"}`
	summary, err := uc.GeneratePatientSummary(ctx, operatorID, subjectID, medicalJSON, "en")
	assert.NoError(t, err)
	assert.Contains(t, summary, "blood sugar has stabilized")
	assert.Contains(t, summary, "Metformin")

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionReadPHI, record.Action)
	assert.Equal(t, "PatientSummary", record.ResourceType)

	// 2. Check Hypertension summary triggers (Hindi translation)
	medicalJSON = `{"medications": [{"name": "Amlodipine"}], "diagnosis": "hypertension"}`
	summary, err = uc.GeneratePatientSummary(ctx, operatorID, subjectID, medicalJSON, "hi")
	assert.NoError(t, err)
	assert.Contains(t, summary, "आपकी माताजी का रक्तचाप")
	assert.Contains(t, summary, "Amlodipine")

	// 3. Check Portuguese translation
	summary, err = uc.GeneratePatientSummary(ctx, operatorID, subjectID, medicalJSON, "pt")
	assert.NoError(t, err)
	assert.Contains(t, summary, "A tensão arterial da sua mãe")
}

func TestAnalyzeTelemetryTrend(t *testing.T) {
	ctx := context.Background()
	repo := newMockTelemetryRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	llm := NewMockLLMClient()
	uc := NewHealthUseCase(repo, llm, logger)

	operatorID := uint(10)
	subjectID := uint(2)

	// 1. Test empty telemetry
	report, err := uc.AnalyzeTelemetryTrend(ctx, operatorID, subjectID, "blood_pressure", 10, "en")
	assert.NoError(t, err)
	assert.Equal(t, "No vitals telemetry data available to analyze.", report)

	// 2. Test stable baseline (Portuguese)
	_ = repo.RecordVitals(ctx, &model.VitalTelemetry{SubjectID: subjectID, VitalType: "blood_pressure", ValueMetric: 120.0, RecordedAt: time.Now()})
	report, err = uc.AnalyzeTelemetryTrend(ctx, operatorID, subjectID, "blood_pressure", 10, "pt")
	assert.NoError(t, err)
	assert.Contains(t, report, "Os sinais vitais estão estáveis")

	// 3. Test high blood pressure trend trigger (Hindi)
	_ = repo.RecordVitals(ctx, &model.VitalTelemetry{SubjectID: subjectID, VitalType: "blood_pressure", ValueMetric: 142.0, RecordedAt: time.Now()})
	report, err = uc.AnalyzeTelemetryTrend(ctx, operatorID, subjectID, "blood_pressure", 10, "hi")
	assert.NoError(t, err)
	assert.Contains(t, report, "पिछले 7 दिनों में सिस्टोलिक रक्तचाप")
	assert.Contains(t, report, "नमस्ते माँ, आज आप कैसा महसूस कर रही हैं?")
}

func TestResolveDrugEquivalency(t *testing.T) {
	ctx := context.Background()
	repo := newMockTelemetryRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	llm := NewMockLLMClient()
	uc := NewHealthUseCase(repo, llm, logger)

	operatorID := uint(10)

	// 1. Glycomet equivalence
	res, err := uc.ResolveDrugEquivalency(ctx, operatorID, "Glycomet 500mg", "India")
	assert.NoError(t, err)
	assert.Contains(t, res, "Metformin Hydrochloride")
	assert.Contains(t, res, "Glucophage")

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionReadPHI, record.Action)
	assert.Equal(t, uint(0), record.TargetPatientID)
	assert.Equal(t, "DrugEquivalency", record.ResourceType)

	// 2. Aldactone equivalence
	res, err = uc.ResolveDrugEquivalency(ctx, operatorID, "Aldactone 25mg", "India")
	assert.NoError(t, err)
	assert.Contains(t, res, "Spironolactone")
}
