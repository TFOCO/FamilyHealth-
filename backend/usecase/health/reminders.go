package health

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
)

// TimezoneDiscrepancy holds the details of offset differences between parent and sponsor.
type TimezoneDiscrepancy struct {
	ParentTimezone  string        `json:"parent_timezone"`
	SponsorTimezone string        `json:"sponsor_timezone"`
	ParentOffset    int           `json:"parent_offset_seconds"`
	SponsorOffset   int           `json:"sponsor_offset_seconds"`
	Difference      time.Duration `json:"difference"`
}

// AlertNotifier defines an interface for sending notifications/alerts.
type AlertNotifier interface {
	SendPushNotification(ctx context.Context, subjectID uint, message string) error
	SendMissedLogAlert(ctx context.Context, subjectID uint, sponsorID uint, message string) error
}

// ReminderUseCase coordinates timezone-aware reminders and alerts.
type ReminderUseCase interface {
	CalculateTimeDiscrepancy(parentTZ, sponsorTZ string, refTime time.Time) (*TimezoneDiscrepancy, error)
	TriggerLocalPushSchedule(ctx context.Context, subjectID uint, parentTZ string, preferredLocalHour int, refTime time.Time) (time.Time, error)
	TriggerMissedLogAlertRoutine(ctx context.Context, subjectID uint, sponsorID uint, parentTZ string, deadlineLocalHour int, refTime time.Time) (bool, error)
}

type reminderUseCase struct {
	telemetryRepo repository.TelemetryRepository
	notifier      AlertNotifier
}

// NewReminderUseCase creates a new instance of ReminderUseCase.
func NewReminderUseCase(telemetryRepo repository.TelemetryRepository, notifier AlertNotifier) ReminderUseCase {
	return &reminderUseCase{
		telemetryRepo: telemetryRepo,
		notifier:      notifier,
	}
}

// resolveTimezone maps user-friendly abbreviations to valid IANA timezone names.
func resolveTimezone(tz string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(tz))
	switch cleaned {
	case "IST":
		return "Asia/Kolkata"
	case "PORTUGAL":
		return "Europe/Lisbon"
	case "USD":
		// Fallback for USD zone to America/New_York (EST/EDT)
		return "America/New_York"
	case "EUR":
		// Fallback for EUR zone to Europe/Paris (CET/CEST)
		return "Europe/Paris"
	default:
		return tz
	}
}

// CalculateTimeDiscrepancy calculates offset discrepancy between parent and sponsor zones.
func (u *reminderUseCase) CalculateTimeDiscrepancy(parentTZ, sponsorTZ string, refTime time.Time) (*TimezoneDiscrepancy, error) {
	resolvedParent := resolveTimezone(parentTZ)
	resolvedSponsor := resolveTimezone(sponsorTZ)

	parentLoc, err := time.LoadLocation(resolvedParent)
	if err != nil {
		return nil, fmt.Errorf("failed to load parent location '%s' (resolved '%s'): %w", parentTZ, resolvedParent, err)
	}

	sponsorLoc, err := time.LoadLocation(resolvedSponsor)
	if err != nil {
		return nil, fmt.Errorf("failed to load sponsor location '%s' (resolved '%s'): %w", sponsorTZ, resolvedSponsor, err)
	}

	pTime := refTime.In(parentLoc)
	sTime := refTime.In(sponsorLoc)

	_, pOffset := pTime.Zone()
	_, sOffset := sTime.Zone()

	diffSeconds := pOffset - sOffset

	return &TimezoneDiscrepancy{
		ParentTimezone:  resolvedParent,
		SponsorTimezone: resolvedSponsor,
		ParentOffset:    pOffset,
		SponsorOffset:   sOffset,
		Difference:      time.Duration(diffSeconds) * time.Second,
	}, nil
}

// TriggerLocalPushSchedule computes the next local notification schedule for the parent,
// triggers the push notification via notifier if refTime matches the schedule,
// and returns the target push time in UTC.
func (u *reminderUseCase) TriggerLocalPushSchedule(ctx context.Context, subjectID uint, parentTZ string, preferredLocalHour int, refTime time.Time) (time.Time, error) {
	resolvedParent := resolveTimezone(parentTZ)
	parentLoc, err := time.LoadLocation(resolvedParent)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load timezone '%s': %w", parentTZ, err)
	}

	localRef := refTime.In(parentLoc)

	// Calculate target push time for today in parent's local time
	targetPushTime := time.Date(localRef.Year(), localRef.Month(), localRef.Day(), preferredLocalHour, 0, 0, 0, parentLoc)

	// If current time is after the push time today, the next schedule is tomorrow
	nextPushUTC := targetPushTime.UTC()
	if localRef.After(targetPushTime) || localRef.Equal(targetPushTime) {
		nextPushUTC = targetPushTime.AddDate(0, 0, 1).UTC()
	}

	// Trigger notifier if the current time matches the scheduled push hour today
	if localRef.Hour() == preferredLocalHour && localRef.Minute() == 0 {
		if u.notifier != nil {
			msg := fmt.Sprintf("Reminder: It is %d:00 local time. Please record your daily health vitals.", preferredLocalHour)
			_ = u.notifier.SendPushNotification(ctx, subjectID, msg)
		}
	}

	return nextPushUTC, nil
}

// TriggerMissedLogAlertRoutine checks if the parent has logged their vitals by a certain local deadline hour.
// If the deadline is passed, and no vitals exist for the parent's local calendar day, it triggers an alert
// to the sponsor and returns true. Otherwise, it returns false.
func (u *reminderUseCase) TriggerMissedLogAlertRoutine(ctx context.Context, subjectID uint, sponsorID uint, parentTZ string, deadlineLocalHour int, refTime time.Time) (bool, error) {
	resolvedParent := resolveTimezone(parentTZ)
	parentLoc, err := time.LoadLocation(resolvedParent)
	if err != nil {
		return false, fmt.Errorf("failed to load timezone '%s': %w", parentTZ, err)
	}

	localRef := refTime.In(parentLoc)
	startOfLocalDay := time.Date(localRef.Year(), localRef.Month(), localRef.Day(), 0, 0, 0, 0, parentLoc)
	endOfLocalDay := startOfLocalDay.AddDate(0, 0, 1)

	// Get vitals for the subject
	vitals, err := u.telemetryRepo.ListVitals(ctx, subjectID, "", 100)
	if err != nil {
		return false, fmt.Errorf("failed to fetch vitals: %w", err)
	}

	// Check if there are any vital telemetry entries logged during the local calendar day
	hasLoggedToday := false
	for _, v := range vitals {
		vLocal := v.RecordedAt.In(parentLoc)
		if (vLocal.Equal(startOfLocalDay) || vLocal.After(startOfLocalDay)) && vLocal.Before(endOfLocalDay) {
			hasLoggedToday = true
			break
		}
	}

	// Deadline for today
	deadline := time.Date(localRef.Year(), localRef.Month(), localRef.Day(), deadlineLocalHour, 0, 0, 0, parentLoc)

	// If they have not logged today, and we are past the local deadline, trigger the alert routine
	if !hasLoggedToday && (localRef.After(deadline) || localRef.Equal(deadline)) {
		if u.notifier != nil {
			msg := fmt.Sprintf("Alert: Your parent (ID %d) has missed their daily health log. Deadline was %d:00 parent local time.", subjectID, deadlineLocalHour)
			err = u.notifier.SendMissedLogAlert(ctx, subjectID, sponsorID, msg)
			if err != nil {
				return true, fmt.Errorf("failed to send alert notification: %w", err)
			}
		}
		return true, nil
	}

	return false, nil
}
