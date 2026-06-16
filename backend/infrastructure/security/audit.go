package security

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// AuditAction defines the type of security events that must be tracked for HIPAA/DPDP.
type AuditAction string

const (
	ActionReadPHI   AuditAction = "READ_PHI"
	ActionWritePHI  AuditAction = "WRITE_PHI"
	ActionDeletePHI AuditAction = "DELETE_PHI"
	ActionAuthFail  AuditAction = "AUTH_FAIL"
)

// AuditRecord represents a structured compliance audit trail record.
type AuditRecord struct {
	Timestamp       time.Time   `json:"timestamp"`
	OperatorID      uint        `json:"operator_id"`
	Action          AuditAction `json:"action"`
	TargetPatientID uint        `json:"target_patient_id"`
	ResourceType    string      `json:"resource_type"`
	IPAddress       string      `json:"ip_address"`
	UserAgent       string      `json:"user_agent"`
}

// AuditLogger handles secure write-only logging of sensitive actions.
type AuditLogger struct {
	writer io.Writer
}

// NewAuditLogger initializes a compliance logger writing to the specified writer.
func NewAuditLogger(writer io.Writer) *AuditLogger {
	if writer == nil {
		writer = os.Stdout
	}
	return &AuditLogger{writer: writer}
}

// Log writes a compliance record to the audit stream.
func (l *AuditLogger) Log(operatorID uint, action AuditAction, targetPatientID uint, resourceType, ip, userAgent string) error {
	record := AuditRecord{
		Timestamp:       time.Now().UTC(),
		OperatorID:      operatorID,
		Action:          action,
		TargetPatientID: targetPatientID,
		ResourceType:    resourceType,
		IPAddress:       ip,
		UserAgent:       userAgent,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal audit record: %w", err)
	}

	// Append a newline for log stream processors
	_, err = fmt.Fprintln(l.writer, string(data))
	if err != nil {
		return fmt.Errorf("failed to write audit record to stream: %w", err)
	}

	return nil
}
