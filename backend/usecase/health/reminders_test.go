package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
)

type mockAlertNotifier struct {
	pushNotifications []struct {
		subjectID uint
		message   string
	}
	missedLogAlerts []struct {
		subjectID uint
		sponsorID uint
		message   string
	}
}

func (m *mockAlertNotifier) SendPushNotification(ctx context.Context, subjectID uint, message string) error {
	m.pushNotifications = append(m.pushNotifications, struct {
		subjectID uint
		message   string
	}{subjectID, message})
	return nil
}

func (m *mockAlertNotifier) SendMissedLogAlert(ctx context.Context, subjectID uint, sponsorID uint, message string) error {
	m.missedLogAlerts = append(m.missedLogAlerts, struct {
		subjectID uint
		sponsorID uint
		message   string
	}{subjectID, sponsorID, message})
	return nil
}

func TestReminderUseCase_CalculateTimeDiscrepancy(t *testing.T) {
	repo := newMockTelemetryRepository()
	notifier := &mockAlertNotifier{}
	uc := NewReminderUseCase(repo, notifier)

	// Test reference time: 2026-06-16T12:00:00Z
	refTime := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	// Parent: IST (UTC+5:30) vs Sponsor: USD/EST (UTC-4:00 during DST in June)
	// Offset difference should be: +5.5 hours - (-4 hours) = +9.5 hours
	disc, err := uc.CalculateTimeDiscrepancy("IST", "USD", refTime)
	assert.NoError(t, err)
	assert.Equal(t, "Asia/Kolkata", disc.ParentTimezone)
	assert.Equal(t, "America/New_York", disc.SponsorTimezone)
	assert.Equal(t, 19800, disc.ParentOffset) // 5.5 * 3600
	assert.Equal(t, -14400, disc.SponsorOffset) // -4 * 3600
	assert.Equal(t, 9*time.Hour+30*time.Minute, disc.Difference)

	// Parent: Portugal (UTC+1 during DST in June) vs Sponsor: EUR/Paris (UTC+2 during DST)
	// Offset difference: +1 hour - (+2 hours) = -1 hour
	disc2, err := uc.CalculateTimeDiscrepancy("Portugal", "EUR", refTime)
	assert.NoError(t, err)
	assert.Equal(t, "Europe/Lisbon", disc2.ParentTimezone)
	assert.Equal(t, "Europe/Paris", disc2.SponsorTimezone)
	assert.Equal(t, 3600, disc2.ParentOffset)
	assert.Equal(t, 7200, disc2.SponsorOffset)
	assert.Equal(t, -1*time.Hour, disc2.Difference)
}

func TestReminderUseCase_TriggerLocalPushSchedule(t *testing.T) {
	repo := newMockTelemetryRepository()
	notifier := &mockAlertNotifier{}
	uc := NewReminderUseCase(repo, notifier)

	// Ref time: 2026-06-16T07:30:00 local time in Portugal (UTC+1 in June) -> UTC: 2026-06-16T06:30:00Z
	portugalLoc, err := time.LoadLocation("Europe/Lisbon")
	assert.NoError(t, err)
	refTime := time.Date(2026, 6, 16, 7, 30, 0, 0, portugalLoc)

	// Target push is 8 AM local time (today)
	nextPush, err := uc.TriggerLocalPushSchedule(context.Background(), 1, "Portugal", 8, refTime)
	assert.NoError(t, err)
	// Target should be today at 8:00 AM Lisbon time
	expectedPush := time.Date(2026, 6, 16, 8, 0, 0, 0, portugalLoc).UTC()
	assert.Equal(t, expectedPush, nextPush)
	assert.Empty(t, notifier.pushNotifications)

	// Now try at exactly 8:00 AM local time -> push notification should trigger
	refTimePush := time.Date(2026, 6, 16, 8, 0, 0, 0, portugalLoc)
	nextPush2, err := uc.TriggerLocalPushSchedule(context.Background(), 1, "Portugal", 8, refTimePush)
	assert.NoError(t, err)
	// Next push should roll over to tomorrow 8:00 AM Lisbon time
	expectedPush2 := time.Date(2026, 6, 17, 8, 0, 0, 0, portugalLoc).UTC()
	assert.Equal(t, expectedPush2, nextPush2)
	assert.Len(t, notifier.pushNotifications, 1)
	assert.Equal(t, uint(1), notifier.pushNotifications[0].subjectID)
	assert.Contains(t, notifier.pushNotifications[0].message, "Reminder: It is 8:00 local time")
}

func TestReminderUseCase_TriggerMissedLogAlertRoutine(t *testing.T) {
	repo := newMockTelemetryRepository()
	notifier := &mockAlertNotifier{}
	uc := NewReminderUseCase(repo, notifier)

	subjectID := uint(2)
	sponsorID := uint(1)
	portugalLoc, err := time.LoadLocation("Europe/Lisbon")
	assert.NoError(t, err)

	// Deadline: 21:00 (9 PM)
	// Scenario A: Before deadline (e.g., 20:00), no logs yet -> Alert should NOT trigger
	refTimeBefore := time.Date(2026, 6, 16, 20, 0, 0, 0, portugalLoc)
	triggered, err := uc.TriggerMissedLogAlertRoutine(context.Background(), subjectID, sponsorID, "Portugal", 21, refTimeBefore)
	assert.NoError(t, err)
	assert.False(t, triggered)
	assert.Empty(t, notifier.missedLogAlerts)

	// Scenario B: After deadline (e.g., 22:00), no logs yet -> Alert SHOULD trigger
	refTimeAfter := time.Date(2026, 6, 16, 22, 0, 0, 0, portugalLoc)
	triggered, err = uc.TriggerMissedLogAlertRoutine(context.Background(), subjectID, sponsorID, "Portugal", 21, refTimeAfter)
	assert.NoError(t, err)
	assert.True(t, triggered)
	assert.Len(t, notifier.missedLogAlerts, 1)
	assert.Equal(t, subjectID, notifier.missedLogAlerts[0].subjectID)
	assert.Equal(t, sponsorID, notifier.missedLogAlerts[0].sponsorID)
	assert.Contains(t, notifier.missedLogAlerts[0].message, "has missed their daily health log")

	// Scenario C: After deadline (e.g., 22:00), but has a log today -> Alert should NOT trigger
	notifier.missedLogAlerts = nil // reset
	// Log time today at 10:00 AM local time (9:00 AM UTC)
	logTimeToday := time.Date(2026, 6, 16, 10, 0, 0, 0, portugalLoc).UTC()
	err = repo.RecordVitals(context.Background(), &model.VitalTelemetry{
		SubjectID:  subjectID,
		RecordedAt: logTimeToday,
	})
	assert.NoError(t, err)

	triggered, err = uc.TriggerMissedLogAlertRoutine(context.Background(), subjectID, sponsorID, "Portugal", 21, refTimeAfter)
	assert.NoError(t, err)
	assert.False(t, triggered)
	assert.Empty(t, notifier.missedLogAlerts)
}
