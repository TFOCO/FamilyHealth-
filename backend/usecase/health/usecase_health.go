package health

import (
	"context"
	"fmt"
	"strings"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// LLMClient defines the interface for interacting with the AI service, supporting language selection.
type LLMClient interface {
	GenerateDiasporaSummary(ctx context.Context, medicalJSON string, lang string) (string, error)
	AnalyzeTelemetryTrend(ctx context.Context, telemetryData string, lang string) (string, error)
	ResolveDrugEquivalency(ctx context.Context, brandName, region string) (string, error)
}

// MockLLMClient implements LLMClient with multilingual rule-based mock responses derived from PROMPT.md.
type MockLLMClient struct{}

// NewMockLLMClient creates a new mock instance of LLMClient.
func NewMockLLMClient() LLMClient {
	return &MockLLMClient{}
}

// GenerateDiasporaSummary mock implementation with translation.
func (m *MockLLMClient) GenerateDiasporaSummary(ctx context.Context, medicalJSON string, lang string) (string, error) {
	lang = strings.ToLower(lang)
	var overallStatus, medsAdjust, nextCall, actionItems string

	isDiabetes := strings.Contains(medicalJSON, "Metformin") || strings.Contains(medicalJSON, "diabetic") || strings.Contains(medicalJSON, "glucose")
	isHypertension := strings.Contains(medicalJSON, "Amlodipine") || strings.Contains(medicalJSON, "blood_pressure") || strings.Contains(medicalJSON, "hypertension")

	if strings.Contains(lang, "hi") || strings.Contains(lang, "hin") {
		// Hindi translation
		if isDiabetes {
			overallStatus = "आपके पिता का ब्लड शुगर 110 mg/dL पर स्थिर हो गया है। डॉक्टर ने कहा कि उनका मधुमेह वर्तमान में अच्छी तरह से नियंत्रित है।"
			medsAdjust = "उन्हें मेटफॉर्मिन 500mg लेना जारी रखना चाहिए। डॉक्टर ने पेट की संवेदनशीलता को कम करने के लिए इसे नाश्ते के बाद लेने की सिफारिश की है।"
			nextCall = "- उनसे पूछें कि क्या उन्होंने आज नाश्ते के बाद मेटफॉर्मिन लिया था।\n- पूछें कि क्या उन्हें कोई हल्की मतली या पेट में बेचैनी महसूस हुई।"
			actionItems = "- जांचें कि क्या उनके पास अगले 30 दिनों के लिए मेटफॉर्मिन की ताज़ा आपूर्ति है।"
		} else if isHypertension {
			overallStatus = "आपकी माताजी का रक्तचाप आज थोड़ा बढ़ा हुआ है, लेकिन वे अन्यथा ठीक महसूस कर रही हैं। डॉक्टर ने उनकी दवा में बदलाव किया है।"
			medsAdjust = "डॉक्टर ने दिन में एक बार Tab. Amlodipine 5mg जोड़ा है। कृपया सुनिश्चित करें कि वे डॉक्टर के निर्देशानुसार पुरानी रक्तचाप की दवाएं बंद कर दें।"
			nextCall = "- माताजी से पूछें कि क्या उनके पैरों या टखनों में कोई सूजन है।\n- पूछें कि क्या उन्हें खड़े होने पर कोई चक्कर आया है।"
			actionItems = "- स्थानीय फार्मेसी से एम्लोडिपाइन का पर्चा फिर से भरवाएं।\n- 2 सप्ताह में हृदय रोग विशेषज्ञ से जांच का कार्यक्रम निर्धारित करें।"
		} else {
			overallStatus = "आपके माता-पिता का स्वास्थ्य स्थिर है। डॉक्टर उनकी समग्र स्थिति और प्रगति से प्रसन्न थे।"
			medsAdjust = "उनकी वर्तमान दवा अनुसूची में कोई बड़ा बदलाव नहीं किया गया है। हमेशा की तरह अपनी सक्रिय दवाएं जारी रखें।"
			nextCall = "- पूछें कि वे आज कैसा महसूस कर रहे हैं।\n- पूछें कि क्या उन्हें सप्ताह के लिए अपनी दवा आयोजक स्थापित करने में सहायता की आवश्यकता है।"
			actionItems = "- उनके दैनिक स्वास्थ्य लॉग की निगरानी करें।\n- सुनिश्चित करें कि वे सक्रिय और हाइड्रेटेड रहें।"
		}
	} else if strings.Contains(lang, "pt") || strings.Contains(lang, "por") {
		// Portuguese translation
		if isDiabetes {
			overallStatus = "O açúcar no sangue do seu pai estabilizou em 110 mg/dL. O médico observou que o diabetes dele está bem controlado."
			medsAdjust = "Ele deve continuar a tomar Metformina 500mg. O médico recomendou tomar após o pequeno-almoço para reduzir a sensibilidade estomacal."
			nextCall = "- Pergunte se ele tomou a Metformina após o pequeno-almoço hoje.\n- Pergunte se ele teve alguma náusea leve ou desconforto estomacal."
			actionItems = "- Verifique se ele tem Metformina suficiente para os próximos 30 dias."
		} else if isHypertension {
			overallStatus = "A tensão arterial da sua mãe está ligeiramente elevada hoje, mas ela sente-se bem. O médico ajustou a medicação."
			medsAdjust = "O médico adicionou Amlodipina 5mg uma vez ao dia. Certifique-se de que ela interrompe os medicamentos antigos conforme indicado."
			nextCall = "- Pergunte à mãe se ela teve algum inchaço nos pés ou tornozelos.\n- Pergunte se ela sentiu tonturas ao levantar-se."
			actionItems = "- Obtenha a receita de Amlodipina na farmácia local.\n- Agende uma consulta com o cardiologista em 2 semanas."
		} else {
			overallStatus = "O estado de saúde dos seus pais está estável. O médico ficou satisfeito com a sua condição geral."
			medsAdjust = "Sem alterações na medicação atual. Continue com as receitas ativas normalmente."
			nextCall = "- Pergunte como se sentem hoje.\n- Pergunte se precisam de ajuda para organizar os medicamentos da semana."
			actionItems = "- Monitorize os registos diários de saúde.\n- Garanta que se mantêm ativos e hidratados."
		}
	} else {
		// English default
		if isDiabetes {
			overallStatus = "Your father's blood sugar has stabilized at 110 mg/dL. The doctor noted that his diabetes is currently well-managed."
			medsAdjust = "He should continue taking Metformin 500mg. The doctor recommended taking it after breakfast to reduce stomach sensitivity."
			nextCall = "- Ask him if he took his Metformin after breakfast today.\n- Ask if he has had any mild nausea or stomach discomfort."
			actionItems = "- Verify that he has a fresh supply of Metformin for the next 30 days."
		} else if isHypertension {
			overallStatus = "Your mother's blood pressure is slightly elevated today, but she is otherwise feeling well. The doctor has adjusted her medication."
			medsAdjust = "The doctor added Tab. Amlodipine 5mg once daily. Please make sure she stops any old blood pressure pills if instructed by her doctor."
			nextCall = "- Ask mom if she has had any swelling in her feet or ankles.\n- Ask if she has experienced any dizziness when standing up."
			actionItems = "- Refill the Amlodipine prescription at the local pharmacy.\n- Schedule a follow-up cardiologist checkup in 2 weeks."
		} else {
			overallStatus = "Your parent's health status is stable. The doctor was pleased with their overall condition and progress."
			medsAdjust = "No major changes to their current medication schedule have been made. Continue their active prescriptions as usual."
			nextCall = "- Ask how they are feeling generally today.\n- Ask if they need help setting up their pill organizer for the week."
			actionItems = "- Monitor their daily health logs.\n- Ensure they stay active and hydrated."
		}
	}

	summary := fmt.Sprintf(
		"### 1. Overall Status\n%s\n\n### 2. Medication Adjustments\n%s\n\n### 3. What to Ask on Your Next Call\n%s\n\n### 4. Action Items\n%s",
		overallStatus, medsAdjust, nextCall, actionItems,
	)
	return summary, nil
}

// AnalyzeTelemetryTrend mock implementation with translation.
func (m *MockLLMClient) AnalyzeTelemetryTrend(ctx context.Context, telemetryData string, lang string) (string, error) {
	lang = strings.ToLower(lang)
	isHighBP := strings.Contains(telemetryData, "142") || strings.Contains(telemetryData, "140") || strings.Contains(telemetryData, "145")

	var trendExplanation, script, recommendation string

	if strings.Contains(lang, "hi") || strings.Contains(lang, "hin") {
		// Hindi translation
		if isHighBP {
			trendExplanation = "पिछले 7 दिनों में सिस्टोलिक रक्तचाप 125 mmHg के स्थिर औसत से बढ़कर 142 mmHg हो गया है, जो सामान्य सीमाओं से बाहर मध्यम वृद्धि का संकेत देता है।"
			script = "\"नमस्ते माँ, आज आप कैसा महसूस कर रही हैं? बस यह पूछना चाहता था कि क्या आप नियमित रूप से बीपी की गोली ले रही हैं, और क्या आपको कोई चक्कर या सिरदर्द महसूस हो रहा है?\""
			recommendation = "हम आपकी माताजी की रक्तचाप की दवा की खुराक की समीक्षा के लिए स्थानीय चिकित्सक से संपर्क करने की दृढ़ता से सिफारिश करते हैं।"
		} else {
			trendExplanation = "महत्वपूर्ण लक्षण सामान्य सीमा के भीतर स्थिर हैं।"
			script = "\"नमस्ते पिताजी, आशा है आपका दिन अच्छा बीत रहा होगा! आपके स्वास्थ्य लॉग देखे और सब कुछ बहुत बढ़िया है। आज आप कैसा महसूस कर रहे हैं?\""
			recommendation = "वर्तमान दिनचर्या और नियमित दैनिक लॉग जारी रखें।"
		}
	} else if strings.Contains(lang, "pt") || strings.Contains(lang, "por") {
		// Portuguese translation
		if isHighBP {
			trendExplanation = "A tensão arterial sistólica subiu de uma média estável de 125 mmHg para 142 mmHg nos últimos 7 dias, indicando uma tendência ascendente moderada fora dos limites normais."
			script = "\"Olá Mãe, como te sentes hoje? Só queria saber se tens tomado o comprimido da tensão regularmente e se tens sentido tonturas ou dores de cabeça?\""
			recommendation = "Recomendamos vivamente que contacte o médico assistente para rever a dosagem da medicação para a tensão arterial."
		} else {
			trendExplanation = "Os sinais vitais estão estáveis e dentro dos limites normais de referência."
			script = "\"Olá Pai, espero que estejas a ter um bom dia! Verifiquei os teus registos e está tudo ótimo. Como te sentes hoje?\""
			recommendation = "Continue com a rotina atual e os registos diários de telemetria."
		}
	} else {
		// English default
		if isHighBP {
			trendExplanation = "Systolic blood pressure has risen from a stable average of 125 mmHg to 142 mmHg over the past 7 days, indicating a moderate upward trend outside normal limits."
			script = "\"Hi Ma, how are you feeling today? Just wanted to check if you've been taking the BP pill regularly, and whether you're feeling any dizziness or headaches?\""
			recommendation = "We strongly recommend contacting your local physician to review her blood pressure medication dosage."
		} else {
			trendExplanation = "Vitals are stable and within normal baseline limits. No significant upward or downward trends detected."
			script = "\"Hi Dad, hope you're having a good day! Checked your logs and everything looks great. How are you feeling today?\""
			recommendation = "Continue with the current routine and regular daily telemetry logs."
		}
	}

	report := fmt.Sprintf(
		"### AI Telemetry Trend Watchdog Report\n\n**Trend Analysis**:\n%s\n\n**Suggested Check-in Script**:\n%s\n\n**Recommendation**:\n%s",
		trendExplanation, script, recommendation,
	)
	return report, nil
}

// ResolveDrugEquivalency mock implementation.
func (m *MockLLMClient) ResolveDrugEquivalency(ctx context.Context, brandName, region string) (string, error) {
	var generic, class, indication, equivalents, note string
	brandNameUpper := strings.ToUpper(brandName)

	if strings.Contains(brandNameUpper, "GLYCOMET") {
		generic = "Metformin Hydrochloride"
		class = "Biguanide (Oral Hypoglycemic)"
		indication = "Type 2 Diabetes Mellitus"
		equivalents = "*   **USA**: Glucophage, Fortamet, Glumetza\n*   **UK**: Glucophage\n*   **Europe**: Metfogamma, Glucophage"
		note = "If your parent is taking this, make sure they do not combine it with other heavy diabetic medications unless prescribed. Check if they feel any stomach upset or nausea, which are common side effects."
	} else if strings.Contains(brandNameUpper, "ALDACTONE") {
		generic = "Spironolactone"
		class = "Aldosterone Receptor Antagonist (Potassium-Sparing Diuretic)"
		indication = "Hypertension, Heart Failure, Edema"
		equivalents = "*   **USA**: Aldactone\n*   **UK**: Aldactone\n*   **Europe**: Spiroctan, Practon"
		note = "Ensure they do not take potassium supplements alongside Spironolactone as it can lead to high potassium levels (hyperkalemia). Check if they feel fatigued or experience muscle weakness."
	} else {
		generic = "Unknown / Custom Compound"
		class = "Not Classified"
		indication = "General Therapeutic Indication"
		equivalents = "*   **USA**: Generic equivalent available\n*   **UK**: Consult local formulary\n*   **Europe**: Consult local formulary"
		note = "Please verify the drug chemical names with a local physician before substituting any medication."
	}

	response := fmt.Sprintf(
		"*   **Generic Chemical**: %s\n*   **Drug Class**: %s\n*   **Local Indication**: %s\n*   **International Equivalents**:\n%s\n*   **Sponsor Note**: %s",
		generic, class, indication, equivalents, note,
	)
	return response, nil
}

// HealthUseCase defines the orchestrator interface for remote health tracking and clinical summary generation.
type HealthUseCase interface {
	ProcessTelemetry(ctx context.Context, operatorID uint, telemetry *model.VitalTelemetry) error
	ListTelemetry(ctx context.Context, operatorID uint, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error)
	GeneratePatientSummary(ctx context.Context, operatorID uint, subjectID uint, medicalJSON string, lang string) (string, error)
	AnalyzeTelemetryTrend(ctx context.Context, operatorID uint, subjectID uint, vitalType string, limit int, lang string) (string, error)
	ResolveDrugEquivalency(ctx context.Context, operatorID uint, brandName, region string) (string, error)
}

type healthUseCase struct {
	telemetryRepo repository.TelemetryRepository
	llmClient     LLMClient
	auditLogger   *security.AuditLogger
}

// NewHealthUseCase creates a new instance of HealthUseCase.
func NewHealthUseCase(telemetryRepo repository.TelemetryRepository, llmClient LLMClient, auditLogger *security.AuditLogger) HealthUseCase {
	return &healthUseCase{
		telemetryRepo: telemetryRepo,
		llmClient:     llmClient,
		auditLogger:   auditLogger,
	}
}

// ProcessTelemetry records a new vital telemetry reading and logs a WRITE_PHI audit record.
func (u *healthUseCase) ProcessTelemetry(ctx context.Context, operatorID uint, telemetry *model.VitalTelemetry) error {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionWritePHI, telemetry.SubjectID, "VitalTelemetry", "usecase", "backend")
	if err != nil {
		return err
	}

	return u.telemetryRepo.RecordVitals(ctx, telemetry)
}

// ListTelemetry fetches vital telemetry records for a patient and logs a READ_PHI audit record.
func (u *healthUseCase) ListTelemetry(ctx context.Context, operatorID uint, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error) {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, subjectID, "VitalTelemetry", "usecase", "backend")
	if err != nil {
		return nil, err
	}

	return u.telemetryRepo.ListVitals(ctx, subjectID, vitalType, limit)
}

// GeneratePatientSummary triggers an LLM to generate a diaspora family summary and logs a READ_PHI audit record.
func (u *healthUseCase) GeneratePatientSummary(ctx context.Context, operatorID uint, subjectID uint, medicalJSON string, lang string) (string, error) {
	// HIPAA Compliance: Log access event as we are reading PHI to produce summaries
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, subjectID, "PatientSummary", "usecase", "backend")
	if err != nil {
		return "", err
	}

	return u.llmClient.GenerateDiasporaSummary(ctx, medicalJSON, lang)
}

// AnalyzeTelemetryTrend retrieves the latest telemetry readings, formats them, triggers LLM trend analysis, and logs a READ_PHI audit record.
func (u *healthUseCase) AnalyzeTelemetryTrend(ctx context.Context, operatorID uint, subjectID uint, vitalType string, limit int, lang string) (string, error) {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, subjectID, "VitalTelemetryTrend", "usecase", "backend")
	if err != nil {
		return "", err
	}

	telemetry, err := u.telemetryRepo.ListVitals(ctx, subjectID, vitalType, limit)
	if err != nil {
		return "", fmt.Errorf("failed to fetch vitals for trend analysis: %w", err)
	}

	if len(telemetry) == 0 {
		return "No vitals telemetry data available to analyze.", nil
	}

	var builder strings.Builder
	builder.WriteString("Review the last telemetry data:\n")
	for _, v := range telemetry {
		builder.WriteString(fmt.Sprintf("- RecordedAt: %s, Type: %s, Value: %.2f %s, Context: %s\n",
			v.RecordedAt.Format("2006-01-02 15:04:05"),
			v.VitalType,
			v.ValueMetric,
			v.ValueUnit,
			v.ContextData,
		))
	}

	return u.llmClient.AnalyzeTelemetryTrend(ctx, builder.String(), lang)
}

// ResolveDrugEquivalency looks up generic alternatives and equivalent brand names, and logs a READ_PHI audit record.
func (u *healthUseCase) ResolveDrugEquivalency(ctx context.Context, operatorID uint, brandName, region string) (string, error) {
	// HIPAA Compliance: Log access event for reading drug database mappings (non-patient specific but linked to user request context)
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, 0, "DrugEquivalency", "usecase", "backend")
	if err != nil {
		return "", err
	}

	return u.llmClient.ResolveDrugEquivalency(ctx, brandName, region)
}
