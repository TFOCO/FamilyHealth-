package controller

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/gin-gonic/gin"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Emergency Medical Profile | FamilyHealth</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            line-height: 1.6;
            margin: 0;
            padding: 20px;
            background-color: #f8f9fa;
            color: #212529;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            border-top: 5px solid #dc3545;
        }
        h1 {
            color: #dc3545;
            font-size: 26px;
            margin-bottom: 5px;
            display: flex;
            align-items: center;
        }
        .subtitle {
            color: #6c757d;
            font-size: 14px;
            margin-bottom: 25px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .section {
            margin-bottom: 30px;
            padding-bottom: 20px;
            border-bottom: 1px solid #dee2e6;
        }
        .section:last-child {
            border-bottom: none;
            margin-bottom: 0;
            padding-bottom: 0;
        }
        h2 {
            color: #495057;
            font-size: 18px;
            margin-top: 0;
            margin-bottom: 15px;
            border-left: 4px solid #dc3545;
            padding-left: 10px;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 20px;
        }
        .card {
            padding: 15px 20px;
            border-radius: 6px;
            border: 1px solid #dee2e6;
            background: #fff;
        }
        .card-danger {
            background: #fff5f5;
            border-color: #feb2b2;
            color: #9b2c2c;
        }
        .card-warning {
            background: #fffaf0;
            border-color: #fbd38d;
            color: #9c4221;
        }
        .card-info {
            background: #ebf8ff;
            border-color: #bee3f8;
            color: #2b6cb0;
        }
        .card h3 {
            margin: 0 0 8px 0;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: inherit;
            opacity: 0.8;
        }
        .card p {
            margin: 0;
            font-size: 18px;
            font-weight: bold;
        }
        .meds-list {
            background: #f7fafc;
            border: 1px solid #e2e8f0;
            padding: 15px 20px;
            border-radius: 6px;
            white-space: pre-wrap;
            font-family: inherit;
            margin: 0;
            font-size: 15px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 10px;
            font-size: 14px;
        }
        th, td {
            text-align: left;
            padding: 12px;
            border-bottom: 1px solid #e2e8f0;
        }
        th {
            background-color: #f7fafc;
            color: #4a5568;
            font-weight: 600;
        }
        tr:hover {
            background-color: #fcfcfc;
        }
        .badge {
            display: inline-block;
            padding: 4px 8px;
            font-size: 11px;
            font-weight: bold;
            border-radius: 4px;
            text-transform: uppercase;
        }
        .badge-bp { background-color: #fed7d7; color: #9b2c2c; }
        .badge-glucose { background-color: #feebc8; color: #9c4221; }
        .badge-hr { background-color: #e2e8f0; color: #4a5568; }
        .badge-temp { background-color: #ebf8ff; color: #2b6cb0; }
        .badge-generic { background-color: #edf2f7; color: #4a5568; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Emergency Medical Profile</h1>
        <div class="subtitle">Secure Responder Access • Decrypted PHI</div>

        <div class="section">
            <div class="grid">
                <div class="card card-danger">
                    <h3>Blood Group</h3>
                    <p>{{if .BloodGroup}}{{.BloodGroup}}{{else}}Not Provided{{end}}</p>
                </div>
                <div class="card card-warning">
                    <h3>Allergies</h3>
                    <p>{{if .Allergies}}{{.Allergies}}{{else}}No Known Allergies{{end}}</p>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>Active Medications</h2>
            {{if .ActiveMeds}}
                <pre class="meds-list">{{.ActiveMeds}}</pre>
            {{else}}
                <div class="card card-info" style="font-weight: normal;">
                    <p style="font-size: 15px; font-weight: normal; margin: 0;">No active medications documented.</p>
                </div>
            {{end}}
        </div>

        <div class="section">
            <h2>Historical Vitals & Telemetry</h2>
            <table>
                <thead>
                    <tr>
                        <th>Recorded At (UTC)</th>
                        <th>Telemetry Type</th>
                        <th>Measurement</th>
                        <th>Metadata / Notes</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Vitals}}
                    <tr>
                        <td style="color: #718096;">{{.RecordedAt}}</td>
                        <td><span class="badge {{.BadgeClass}}">{{.VitalType}}</span></td>
                        <td><strong>{{.Value}}</strong></td>
                        <td style="color: #4a5568; font-size: 13px;">{{.Details}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="4" style="text-align: center; color: #a0aec0; padding: 20px;">No historical telemetry found.</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`

// DoctorPortalController handles emergency responder access requests
type DoctorPortalController struct {
	emergencyRepo repository.EmergencyRepository
	telemetryRepo repository.TelemetryRepository
	auditLogger   *security.AuditLogger
}

// NewDoctorPortalController creates a new DoctorPortalController instance.
func NewDoctorPortalController(
	emergencyRepo repository.EmergencyRepository,
	telemetryRepo repository.TelemetryRepository,
	auditLogger *security.AuditLogger,
) *DoctorPortalController {
	return &DoctorPortalController{
		emergencyRepo: emergencyRepo,
		telemetryRepo: telemetryRepo,
		auditLogger:   auditLogger,
	}
}

// ResolveAccessCode handles scanning / checking temporary QR hashes.
func (ctrl *DoctorPortalController) ResolveAccessCode(c *gin.Context) {
	qrHash := c.Param("hash")
	if qrHash == "" {
		qrHash = c.Query("hash")
	}

	if qrHash == "" {
		_ = ctrl.auditLogger.Log(
			0,
			security.ActionAuthFail,
			0,
			"DoctorPortal/AccessHashMissing",
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing QR access hash"})
		return
	}

	ctx := c.Request.Context()

	// 1. Fetch & Decrypt Emergency Profile
	qr, err := ctrl.emergencyRepo.ResolveQR(ctx, qrHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query access code"})
		return
	}

	if qr == nil || !qr.IsActive {
		_ = ctrl.auditLogger.Log(
			0,
			security.ActionAuthFail,
			0,
			"DoctorPortal/InvalidHash:"+qrHash,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired access code"})
		return
	}

	// 2. Query Patient Telemetry Vitals History (max 50)
	vitals, err := ctrl.telemetryRepo.ListVitals(ctx, qr.SubjectID, "", 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve patient telemetry"})
		return
	}

	// 3. HIPAA Log the PHI Access Event
	_ = ctrl.auditLogger.Log(
		0, // 0 indicates emergency responder/unauthenticated temporary portal access
		security.ActionReadPHI,
		qr.SubjectID,
		"DoctorPortal/EmergencyQR",
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	// Format vitals for display
	displayVitals := make([]gin.H, len(vitals))
	for i, v := range vitals {
		details := v.ContextData
		var detailsMap map[string]interface{}
		if err := json.Unmarshal([]byte(v.ContextData), &detailsMap); err == nil {
			var pairs []string
			for k, val := range detailsMap {
				pairs = append(pairs, fmt.Sprintf("%s: %v", k, val))
			}
			if len(pairs) > 0 {
				details = strings.Join(pairs, ", ")
			}
		}

		valueStr := fmt.Sprintf("%.1f %s", v.ValueMetric, v.ValueUnit)
		if v.VitalType == "blood_pressure" {
			if detailsMap != nil {
				if dia, ok := detailsMap["diastolic"]; ok {
					valueStr = fmt.Sprintf("%.0f/%.0f %s", v.ValueMetric, dia, v.ValueUnit)
				}
			}
		}

		badgeClass := "badge-generic"
		switch v.VitalType {
		case "blood_pressure":
			badgeClass = "badge-bp"
		case "blood_glucose":
			badgeClass = "badge-glucose"
		case "heart_rate":
			badgeClass = "badge-hr"
		case "temperature":
			badgeClass = "badge-temp"
		}

		displayVitals[i] = gin.H{
			"RecordedAt": v.RecordedAt.Format("2006-01-02 15:04:05"),
			"VitalType":  formatVitalType(v.VitalType),
			"Value":      valueStr,
			"Details":    details,
			"BadgeClass": badgeClass,
		}
	}

	// 4. Negotiate Content Type (JSON vs HTML)
	acceptHeader := c.GetHeader("Accept")
	wantsHTML := c.Query("format") == "html" ||
		(c.Query("format") != "json" &&
			(strings.Contains(acceptHeader, "text/html") || strings.Contains(acceptHeader, "*/*") || acceptHeader == ""))

	if wantsHTML {
		tmpl, err := template.New("doctor_portal").Parse(htmlTemplate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load template"})
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		_ = tmpl.Execute(c.Writer, gin.H{
			"BloodGroup": qr.BloodGroup,
			"Allergies":  qr.Allergies,
			"ActiveMeds": qr.ActiveMeds,
			"Vitals":     displayVitals,
		})
		return
	}

	// Respond with JSON
	c.JSON(http.StatusOK, gin.H{
		"blood_group": qr.BloodGroup,
		"allergies":   qr.Allergies,
		"active_meds": qr.ActiveMeds,
		"vitals":      vitals,
	})
}

func formatVitalType(vt string) string {
	switch vt {
	case "blood_pressure":
		return "Blood Pressure"
	case "blood_glucose":
		return "Blood Glucose"
	case "heart_rate":
		return "Heart Rate"
	case "temperature":
		return "Temperature"
	case "prescription":
		return "Prescription Media"
	default:
		return strings.Title(strings.ReplaceAll(vt, "_", " "))
	}
}
