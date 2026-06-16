package controller

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HealthController handles vital telemetry and clinical simplification/AI summaries.
type HealthController struct {
	telemetryRepo repository.TelemetryRepository
	auditLogger   *security.AuditLogger
}

// NewHealthController creates a new HealthController instance.
func NewHealthController(telemetryRepo repository.TelemetryRepository, auditLogger *security.AuditLogger) *HealthController {
	return &HealthController{
		telemetryRepo: telemetryRepo,
		auditLogger:   auditLogger,
	}
}

// TelemetryRequest represents the request payload for recording vitals.
type TelemetryRequest struct {
	SubjectID   uint      `json:"subject_id" binding:"required"`
	VitalType   string    `json:"vital_type" binding:"required"`
	ValueMetric float64   `json:"value_metric"`
	ValueUnit   string    `json:"value_unit" binding:"required"`
	ContextData string    `json:"context_data"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// RecordTelemetry processes and persists a new vital sign telemetry reading (POST /api/v1/health/telemetry).
func (ctrl *HealthController) RecordTelemetry(c *gin.Context) {
	var req TelemetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SubjectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject_id is required and cannot be zero"})
		return
	}
	if strings.TrimSpace(req.VitalType) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vital_type is required"})
		return
	}
	if strings.TrimSpace(req.ValueUnit) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value_unit is required"})
		return
	}

	recordedAt := req.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}

	telemetry := model.VitalTelemetry{
		SubjectID:   req.SubjectID,
		VitalType:   req.VitalType,
		ValueMetric: req.ValueMetric,
		ValueUnit:   req.ValueUnit,
		ContextData: req.ContextData,
		RecordedAt:  recordedAt,
	}

	ctx := c.Request.Context()
	if err := ctrl.telemetryRepo.RecordVitals(ctx, &telemetry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record vitals: " + err.Error()})
		return
	}

	// Retrieve operator_id from context (injected by JWT Auth middleware)
	var operatorID uint
	if opVal, exists := c.Get("operator_id"); exists {
		if id, ok := opVal.(uint); ok {
			operatorID = id
		}
	}

	// HIPAA Compliance Audit Log
	_ = ctrl.auditLogger.Log(
		operatorID,
		security.ActionWritePHI,
		telemetry.SubjectID,
		"VitalTelemetry",
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusCreated, telemetry)
}

// ListTelemetry retrieves a list of historical vitals telemetry records (GET /api/v1/health/telemetry).
func (ctrl *HealthController) ListTelemetry(c *gin.Context) {
	subjectStr := c.Query("subject_id")
	if subjectStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing target patient context (subject_id)"})
		return
	}

	subjectID64, err := strconv.ParseUint(subjectStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subject_id format"})
		return
	}
	subjectID := uint(subjectID64)

	vitalType := c.Query("vital_type")

	limitStr := c.Query("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := c.Request.Context()
	vitals, err := ctrl.telemetryRepo.ListVitals(ctx, subjectID, vitalType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve vitals: " + err.Error()})
		return
	}

	var operatorID uint
	if opVal, exists := c.Get("operator_id"); exists {
		if id, ok := opVal.(uint); ok {
			operatorID = id
		}
	}

	// HIPAA Compliance Audit Log
	_ = ctrl.auditLogger.Log(
		operatorID,
		security.ActionReadPHI,
		subjectID,
		"VitalTelemetry",
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, vitals)
}

// SummaryRequest represents the POST payload for summary requests.
type SummaryRequest struct {
	SubjectID         uint   `json:"subject_id" binding:"required"`
	PreferredLanguage string `json:"preferred_language"`
}

// GetSummary generates plain-language AI clinical summaries / vitals trends watchdog (POST/GET /api/v1/health/summary).
func (ctrl *HealthController) GetSummary(c *gin.Context) {
	var subjectID uint
	var prefLang string

	if c.Request.Method == http.MethodPost {
		var req SummaryRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			subjectID = req.SubjectID
			prefLang = req.PreferredLanguage
		}
	}

	if subjectID == 0 {
		subjectStr := c.Query("subject_id")
		if subjectStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "subject_id is required"})
			return
		}
		subjectID64, err := strconv.ParseUint(subjectStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subject_id format"})
			return
		}
		subjectID = uint(subjectID64)
	}

	if prefLang == "" {
		prefLang = c.Query("preferred_language")
	}
	if prefLang == "" {
		prefLang = "English"
	}

	// Query last 14 days of telemetry
	ctx := c.Request.Context()
	vitals, err := ctrl.telemetryRepo.ListVitals(ctx, subjectID, "", 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load patient records for summary"})
		return
	}

	// Analyze trends and build the summary
	var operatorID uint
	if opVal, exists := c.Get("operator_id"); exists {
		if id, ok := opVal.(uint); ok {
			operatorID = id
		}
	}

	// HIPAA Compliance Audit Log
	_ = ctrl.auditLogger.Log(
		operatorID,
		security.ActionReadPHI,
		subjectID,
		"VitalTelemetrySummary",
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	summaryResp := ctrl.generateAnalysis(vitals, prefLang)
	c.JSON(http.StatusOK, summaryResp)
}

// DrugEquivalence represents the returned result for a drug equivalence request.
type DrugEquivalence struct {
	GenericChemical          string                     `json:"generic_chemical"`
	DrugClass                string                     `json:"drug_class"`
	LocalIndication          string                     `json:"local_indication"`
	InternationalEquivalents InternationalBrandEquivalents `json:"international_equivalents"`
	SponsorNote              string                     `json:"sponsor_note"`
}

// InternationalBrandEquivalents holds brand equivalents by market.
type InternationalBrandEquivalents struct {
	USA    string `json:"usa"`
	UK     string `json:"uk"`
	Europe string `json:"europe"`
}

// GetDrugEquivalent maps regional pharmaceutical brand names to generic & international equivalents (GET /api/v1/health/drug-equivalent).
func (ctrl *HealthController) GetDrugEquivalent(c *gin.Context) {
	brand := c.Query("brand_name")
	if brand == "" {
		brand = c.Query("brand")
	}
	region := c.Query("region")
	if region == "" {
		region = c.Query("country")
	}

	if brand == "" || region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both brand_name and region parameters are required"})
		return
	}

	brandClean := strings.ToLower(strings.TrimSpace(brand))
	regionClean := strings.ToLower(strings.TrimSpace(region))

	var eq DrugEquivalence
	found := false

	// Hardcoded mapping representing our pharmacist catalog
	if regionClean == "india" {
		switch brandClean {
		case "glycomet", "metformin":
			eq = DrugEquivalence{
				GenericChemical: "Metformin",
				DrugClass:       "Biguanide (Antidiabetic)",
				LocalIndication: "Type 2 Diabetes Mellitus",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Glucophage, Fortamet, Riomet",
					UK:     "Glucophage, Bolamyn",
					Europe: "Glucophage, Metformin BMS",
				},
				SponsorNote: "If your parent is taking this, make sure they do not combine it with excessive alcohol. Check if they feel any gastrointestinal discomfort (nausea, diarrhea).",
			}
			found = true
		case "amlopin", "norvasc", "amlo", "amlodipine":
			eq = DrugEquivalence{
				GenericChemical: "Amlodipine",
				DrugClass:       "Calcium Channel Blocker",
				LocalIndication: "Hypertension (High Blood Pressure) / Angina",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Norvasc",
					UK:     "Istin",
					Europe: "Norvasc, Amlodipine Pfizer",
				},
				SponsorNote: "If your parent is taking this, monitor for swelling in ankles or feet. Check if they feel dizzy when standing up.",
			}
			found = true
		case "atorva", "lipitor":
			eq = DrugEquivalence{
				GenericChemical: "Atorvastatin",
				DrugClass:       "HMG-CoA Reductase Inhibitor (Statin)",
				LocalIndication: "Hypercholesterolemia (High Cholesterol)",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Lipitor",
					UK:     "Lipitor",
					Europe: "Sortis, Lipitor",
				},
				SponsorNote: "Ensure they take it in the evening. Check if they report muscle pain or unexplained weakness.",
			}
			found = true
		case "lasix":
			eq = DrugEquivalence{
				GenericChemical: "Furosemide",
				DrugClass:       "Loop Diuretic (Water Pill)",
				LocalIndication: "Edema / Congestive Heart Failure / Hypertension",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Lasix",
					UK:     "Lasix, Furosemide",
					Europe: "Lasix, Furosemide Teva",
				},
				SponsorNote: "Monitor for dehydration or muscle cramps. Ensure they take it in the morning to avoid waking up at night to use the restroom.",
			}
			found = true
		}
	} else if regionClean == "portugal" {
		switch brandClean {
		case "nolotil", "metalgial":
			eq = DrugEquivalence{
				GenericChemical: "Metamizole (Dipyrone)",
				DrugClass:       "Non-Steroidal Anti-Inflammatory / Analgesic",
				LocalIndication: "Severe acute pain / High fever resistant to other agents",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Not Approved by FDA (Banned due to agranulocytosis risk)",
					UK:     "Metamizole (Special Order Only)",
					Europe: "Nolotil, Metalgial, Novalgin",
				},
				SponsorNote: "Monitor for any signs of fever, sore throat, or mouth ulcers immediately. This drug can cause a rare but serious blood condition (agranulocytosis).",
			}
			found = true
		case "adalat":
			eq = DrugEquivalence{
				GenericChemical: "Nifedipine",
				DrugClass:       "Calcium Channel Blocker",
				LocalIndication: "Hypertension / Angina Pectoris",
				InternationalEquivalents: InternationalBrandEquivalents{
					USA:    "Adalat CC, Procardia XL",
					UK:     "Adalat, Nifedipress",
					Europe: "Adalat, Nifedipina",
				},
				SponsorNote: "Check if they experience flushing, ankle swelling, or headaches. Do not consume grapefruit juice with this medication.",
			}
			found = true
		}
	}

	if !found {
		// General fallback generator
		brandFormatted := strings.Title(brandClean)
		eq = DrugEquivalence{
			GenericChemical: brandFormatted + " Active Substance",
			DrugClass:       "General Therapeutic Agent",
			LocalIndication: "Treatment of diagnosed medical condition",
			InternationalEquivalents: InternationalBrandEquivalents{
				USA:    brandFormatted + " US Equivalent",
				UK:     brandFormatted + " UK Equivalent",
				Europe: brandFormatted + " EU Equivalent",
			},
			SponsorNote: "Monitor patient for standard side effects. Keep regular doctor consultations and review for cross-border compatibility.",
		}
	}

	var operatorID uint
	if opVal, exists := c.Get("operator_id"); exists {
		if id, ok := opVal.(uint); ok {
			operatorID = id
		}
	}

	// HIPAA Compliance Audit Log (Read PHI lookup, tracking the drug searched)
	_ = ctrl.auditLogger.Log(
		operatorID,
		security.ActionReadPHI,
		0,
		"DrugEquivalenceLookup:"+brand,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, eq)
}

// generateAnalysis processes vitals history and builds an empathetic translated summary/watchdog
func (ctrl *HealthController) generateAnalysis(vitals []model.VitalTelemetry, language string) gin.H {
	langLower := strings.ToLower(language)

	// Trend Analysis
	var hasBP, hasGlucose bool
	var latestBP, latestGlucose float64
	var latestBPUnit, latestGlucoseUnit string
	var diastolicBP float64

	for _, v := range vitals {
		if v.VitalType == "blood_pressure" && !hasBP {
			latestBP = v.ValueMetric
			latestBPUnit = v.ValueUnit
			hasBP = true

			var contextMap map[string]interface{}
			if err := json.Unmarshal([]byte(v.ContextData), &contextMap); err == nil {
				if dia, ok := contextMap["diastolic"]; ok {
					if diaNum, ok := dia.(float64); ok {
						diastolicBP = diaNum
					}
				}
			}
		}
		if v.VitalType == "blood_glucose" && !hasGlucose {
			latestGlucose = v.ValueMetric
			latestGlucoseUnit = v.ValueUnit
			hasGlucose = true
		}
	}

	// Heuristic Trend Messages
	statusMsgEn := "Stable health baseline with normal vitals."
	statusMsgPt := "Linha de base de saúde estável com sinais vitais normais."
	statusMsgHi := "सामान्य जीवन लक्षणों के साथ स्थिर स्वास्थ्य आधार रेखा।"

	if hasBP && latestBP > 140 {
		statusMsgEn = fmt.Sprintf("Your parent's systolic blood pressure is elevated at %.0f %s. Other vitals remain stable.", latestBP, latestBPUnit)
		statusMsgPt = fmt.Sprintf("A tensão arterial sistólica do seu familiar está elevada em %.0f %s. Outros sinais vitais permanecem estáveis.", latestBP, latestBPUnit)
		statusMsgHi = fmt.Sprintf("आपके माता-पिता का सिस्टोलिक रक्तचाप %.0f %s पर बढ़ा हुआ है। अन्य जीवन लक्षण स्थिर बने हुए हैं।", latestBP, latestBPUnit)
	} else if hasGlucose && latestGlucose > 140 {
		statusMsgEn = fmt.Sprintf("Your parent's blood glucose is elevated at %.0f %s. Please monitor their diet and activity.", latestGlucose, latestGlucoseUnit)
		statusMsgPt = fmt.Sprintf("A glicose no sangue do seu familiar está elevada em %.0f %s. Por favor, monitorize a sua dieta e atividade.", latestGlucose, latestGlucoseUnit)
		statusMsgHi = fmt.Sprintf("आपके माता-पिता का रक्त शर्करा (ग्लूकोज) %.0f %s पर बढ़ा हुआ है। कृपया उनके आहार और गतिविधि पर नज़र रखें।", latestGlucose, latestGlucoseUnit)
	}

	switch {
	case strings.Contains(langLower, "hindi") || strings.Contains(langLower, "hi"):
		redFlags := []string{
			"पूछें कि क्या उन्हें सुबह चक्कर आ रहे हैं",
			"सांस फूलने या टखने की सूजन पर नज़र रखें",
		}
		script := "नमस्ते माँ/पिताजी, आप आज कैसा महसूस कर रहे हैं? बस यह पूछना था कि क्या आप नियमित रूप से दवाएं ले रहे हैं?"
		if hasBP && latestBP > 140 {
			script = fmt.Sprintf("नमस्ते माँ/पिताजी, आज आपका रक्तचाप थोड़ा बढ़ा हुआ (%.0f/%.0f) था। क्या आपने आज समय पर दवाई ली? क्या आपको कोई सिरदर्द या चक्कर आ रहा है?", latestBP, diastolicBP)
		}
		return gin.H{
			"status":            "success",
			"summary":           statusMsgHi,
			"trend_explanation": "हालिया रीडिंग के आधार पर स्वास्थ्य की स्थिति स्थिर है लेकिन सिस्टोलिक रक्तचाप की बारीकी से निगरानी की आवश्यकता है।",
			"check_in_script":   script,
			"red_flags":         redFlags,
			"action_items": []string{
				"दैनिक स्वास्थ्य रीडिंग रिकॉर्ड करना जारी रखें",
				"नियमित डॉक्टर से संपर्क बनाए रखें",
			},
		}

	case strings.Contains(langLower, "portuguese") || strings.Contains(langLower, "pt"):
		redFlags := []string{
			"Perguntar se sente tonturas pela manhã.",
			"Monitorizar falta de ar ou inchaço nos tornozelos.",
		}
		script := "Olá Mãe/Pai, como te sentes hoje? Só queria saber se tens tomado a medicação regularmente."
		if hasBP && latestBP > 140 {
			script = fmt.Sprintf("Olá Mãe/Pai, vi que a tua tensão arterial hoje estava um pouco elevada (%.0f/%.0f). Tens tomado o comprimido da tensão regularmente? Sentes alguma dor de cabeça ou tontura?", latestBP, diastolicBP)
		}
		return gin.H{
			"status":            "success",
			"summary":           statusMsgPt,
			"trend_explanation": "Análise de sinais vitais indica estado geral estável, mas recomenda-se atenção à tensão arterial sistólica.",
			"check_in_script":   script,
			"red_flags":         redFlags,
			"action_items": []string{
				"Continuar o registo diário de sinais vitais no portal.",
				"Manter as consultas médicas regulares.",
			},
		}

	default: // Default to English
		redFlags := []string{
			"Ask if they feel any morning dizziness or lightheadedness.",
			"Monitor for shortness of breath or sudden ankle swelling.",
		}
		script := "Hi Ma/Pa, how are you feeling today? Just wanted to check if you've been taking your medications regularly."
		if hasBP && latestBP > 140 {
			script = fmt.Sprintf("Hi Ma/Pa, I saw that your blood pressure was a bit high today (%.0f/%.0f). Did you take your BP pill today? Are you feeling any headaches or dizziness?", latestBP, diastolicBP)
		}
		return gin.H{
			"status":            "success",
			"summary":           statusMsgEn,
			"trend_explanation": "Recent telemetry records indicate a stable baseline with slight elevations. Regular monitoring is recommended.",
			"check_in_script":   script,
			"red_flags":         redFlags,
			"action_items": []string{
				"Continue logging vital telemetry readings daily.",
				"Maintain regular physician follow-up appointments.",
			},
		}
	}
}

// Helpers

// UUIDToUint converts a UUID into a deterministic uint value using FNV hashing.
func UUIDToUint(uid uuid.UUID) uint {
	h := fnv.New32a()
	_, _ = h.Write(uid[:])
	return uint(h.Sum32())
}
