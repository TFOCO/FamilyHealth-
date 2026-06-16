package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, *security.CryptoEngine, *GormRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// A valid 32-byte (64 hex characters) AES key
	hexKey := "6368616e676570617373776f72646d7573746265333262797465736b65792121"
	crypto, err := security.NewCryptoEngine(hexKey)
	require.NoError(t, err)

	repo := NewGormRepository(db, crypto)
	err = repo.Migrate()
	require.NoError(t, err)

	return db, crypto, repo
}

func TestFamilyRepository_CreateLink(t *testing.T) {
	_, _, repo := setupTestDB(t)
	ctx := context.Background()

	link := &model.FamilyLink{
		SponsorID:  1,
		SubjectID:  2,
		Relation:   "Father",
		AccessRole: "admin",
	}

	err := repo.CreateLink(ctx, link)
	assert.NoError(t, err)
	assert.NotZero(t, link.ID)
	assert.False(t, link.CreatedAt.IsZero())

	// Test unique constraint (same sponsor and subject)
	duplicateLink := &model.FamilyLink{
		SponsorID:  1,
		SubjectID:  2,
		Relation:   "Mother",
		AccessRole: "viewer",
	}
	err = repo.CreateLink(ctx, duplicateLink)
	assert.Error(t, err)
}

func TestFamilyRepository_GetLink(t *testing.T) {
	_, _, repo := setupTestDB(t)
	ctx := context.Background()

	// 1. Not found
	link, err := repo.GetLink(ctx, 1, 2)
	assert.NoError(t, err)
	assert.Nil(t, link)

	// 2. Insert and Retrieve
	inserted := &model.FamilyLink{
		SponsorID:  10,
		SubjectID:  20,
		Relation:   "Mother",
		AccessRole: "viewer",
	}
	err = repo.CreateLink(ctx, inserted)
	require.NoError(t, err)

	link, err = repo.GetLink(ctx, 10, 20)
	assert.NoError(t, err)
	assert.NotNil(t, link)
	assert.Equal(t, inserted.ID, link.ID)
	assert.Equal(t, "Mother", link.Relation)
	assert.Equal(t, "viewer", link.AccessRole)
}

func TestFamilyRepository_ListLinksBySponsor(t *testing.T) {
	_, _, repo := setupTestDB(t)
	ctx := context.Background()

	links, err := repo.ListLinksBySponsor(ctx, 100)
	assert.NoError(t, err)
	assert.Empty(t, links)

	link1 := &model.FamilyLink{SponsorID: 100, SubjectID: 101, Relation: "Son", AccessRole: "viewer"}
	link2 := &model.FamilyLink{SponsorID: 100, SubjectID: 102, Relation: "Daughter", AccessRole: "admin"}
	link3 := &model.FamilyLink{SponsorID: 200, SubjectID: 201, Relation: "Spouse", AccessRole: "admin"}

	require.NoError(t, repo.CreateLink(ctx, link1))
	require.NoError(t, repo.CreateLink(ctx, link2))
	require.NoError(t, repo.CreateLink(ctx, link3))

	links, err = repo.ListLinksBySponsor(ctx, 100)
	assert.NoError(t, err)
	assert.Len(t, links, 2)
	assert.ElementsMatch(t, []uint{101, 102}, []uint{links[0].SubjectID, links[1].SubjectID})
}

func TestFamilyRepository_DeleteLink(t *testing.T) {
	_, _, repo := setupTestDB(t)
	ctx := context.Background()

	// Delete non-existing
	err := repo.DeleteLink(ctx, 1, 2)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	// Create and delete
	link := &model.FamilyLink{SponsorID: 5, SubjectID: 6, Relation: "Father", AccessRole: "admin"}
	require.NoError(t, repo.CreateLink(ctx, link))

	err = repo.DeleteLink(ctx, 5, 6)
	assert.NoError(t, err)

	// Verify it's gone
	deleted, err := repo.GetLink(ctx, 5, 6)
	assert.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestTelemetryRepository_RecordAndListVitals(t *testing.T) {
	db, crypto, repo := setupTestDB(t)
	ctx := context.Background()

	sensitiveContext := `{"notes":"Patient was feeling anxious","caffeine_intake":"moderate"}`
	vital := &model.VitalTelemetry{
		SubjectID:   42,
		VitalType:   "blood_pressure",
		ValueMetric: 120.0,
		ValueUnit:   "mmHg",
		ContextData: sensitiveContext,
		RecordedAt:  time.Now().Add(-5 * time.Minute),
	}

	// Record Vitals
	err := repo.RecordVitals(ctx, vital)
	assert.NoError(t, err)
	assert.NotZero(t, vital.ID)

	// 1. Verify that context_data is encrypted in the raw database
	var rawTelemetry VitalTelemetryEntity
	err = db.Table("vital_telemetries").Where("id = ?", vital.ID).First(&rawTelemetry).Error
	require.NoError(t, err)
	assert.NotEmpty(t, rawTelemetry.ContextData)
	assert.NotEqual(t, sensitiveContext, rawTelemetry.ContextData)

	// Manually decrypt raw database field to prove it matches the original plaintext
	decrypted, err := crypto.Decrypt(rawTelemetry.ContextData)
	require.NoError(t, err)
	assert.Equal(t, sensitiveContext, decrypted)

	// 2. Verify ListVitals returns decrypted context data
	vitalsList, err := repo.ListVitals(ctx, 42, "blood_pressure", 10)
	assert.NoError(t, err)
	require.Len(t, vitalsList, 1)
	assert.Equal(t, vital.ID, vitalsList[0].ID)
	assert.Equal(t, sensitiveContext, vitalsList[0].ContextData)
	assert.Equal(t, 120.0, vitalsList[0].ValueMetric)

	// 3. Test vital type filtering
	noMatchList, err := repo.ListVitals(ctx, 42, "blood_glucose", 10)
	assert.NoError(t, err)
	assert.Empty(t, noMatchList)

	// 4. Test Limit and ordering (RecordedAt descending)
	vitalOld := &model.VitalTelemetry{
		SubjectID:   42,
		VitalType:   "blood_pressure",
		ValueMetric: 110.0,
		ValueUnit:   "mmHg",
		ContextData: "old reading",
		RecordedAt:  time.Now().Add(-1 * time.Hour),
	}
	vitalNew := &model.VitalTelemetry{
		SubjectID:   42,
		VitalType:   "blood_pressure",
		ValueMetric: 130.0,
		ValueUnit:   "mmHg",
		ContextData: "new reading",
		RecordedAt:  time.Now(),
	}
	require.NoError(t, repo.RecordVitals(ctx, vitalOld))
	require.NoError(t, repo.RecordVitals(ctx, vitalNew))

	orderedList, err := repo.ListVitals(ctx, 42, "blood_pressure", 2)
	assert.NoError(t, err)
	require.Len(t, orderedList, 2)
	// Newest should be first
	assert.Equal(t, vitalNew.ID, orderedList[0].ID)
	assert.Equal(t, "new reading", orderedList[0].ContextData)
	// Next newest
	assert.Equal(t, vital.ID, orderedList[1].ID)
}

func TestEmergencyRepository_RegisterAndResolveQR(t *testing.T) {
	db, crypto, repo := setupTestDB(t)
	ctx := context.Background()

	activeMeds := `["Lisinopril 10mg daily", "Metformin 500mg bid"]`
	qr := &model.EmergencyQR{
		SubjectID:    99,
		QRHash:       "xyz123hash",
		BloodGroup:   "O-Positive",
		Allergies:    "Penicillin",
		ActiveMeds:   activeMeds,
		SponsorPhone: "+15550199",
		IsActive:     true,
	}

	// Register QR
	err := repo.RegisterQR(ctx, qr)
	assert.NoError(t, err)
	assert.NotZero(t, qr.ID)

	// 1. Verify that active_meds is encrypted in the raw database
	var rawQR EmergencyQREntity
	err = db.Table("emergency_qrs").Where("id = ?", qr.ID).First(&rawQR).Error
	require.NoError(t, err)
	assert.NotEmpty(t, rawQR.ActiveMeds)
	assert.NotEqual(t, activeMeds, rawQR.ActiveMeds)

	// Manually decrypt raw database field to prove it matches the original plaintext
	decrypted, err := crypto.Decrypt(rawQR.ActiveMeds)
	require.NoError(t, err)
	assert.Equal(t, activeMeds, decrypted)

	// 2. Resolve QR and check decrypted data
	resolved, err := repo.ResolveQR(ctx, "xyz123hash")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, qr.ID, resolved.ID)
	assert.Equal(t, activeMeds, resolved.ActiveMeds)
	assert.Equal(t, "O-Positive", resolved.BloodGroup)
	assert.Equal(t, "Penicillin", resolved.Allergies)
	assert.True(t, resolved.IsActive)

	// 3. Resolve QR not found
	nilResolved, err := repo.ResolveQR(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, nilResolved)

	// 4. Update existing profile (RegisterQR performs upsert if ID is provided)
	resolved.ActiveMeds = `["Lisinopril 20mg daily"]`
	err = repo.RegisterQR(ctx, resolved)
	assert.NoError(t, err)

	updated, err := repo.ResolveQR(ctx, "xyz123hash")
	assert.NoError(t, err)
	assert.Equal(t, `["Lisinopril 20mg daily"]`, updated.ActiveMeds)
}

func TestStreakRepository_GetAndSaveStreak(t *testing.T) {
	_, _, repo := setupTestDB(t)
	ctx := context.Background()

	userID := uint(123)

	// 1. Get non-existing streak
	streak, err := repo.GetStreak(ctx, userID)
	assert.NoError(t, err)
	assert.Nil(t, streak)

	// 2. Save new streak
	newStreak := &model.UserStreak{
		UserID:         userID,
		CurrentStreak:  3,
		MaxStreak:      5,
		LastLoggedDate: time.Now().Truncate(time.Second),
	}
	err = repo.SaveStreak(ctx, newStreak)
	assert.NoError(t, err)
	assert.NotZero(t, newStreak.ID)

	// Retrieve and verify
	retrieved, err := repo.GetStreak(ctx, userID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, newStreak.ID, retrieved.ID)
	assert.Equal(t, userID, retrieved.UserID)
	assert.Equal(t, 3, retrieved.CurrentStreak)
	assert.Equal(t, 5, retrieved.MaxStreak)
	assert.True(t, newStreak.LastLoggedDate.Equal(retrieved.LastLoggedDate))

	// 3. Update existing streak
	retrieved.CurrentStreak = 4
	retrieved.MaxStreak = 6
	retrieved.LastLoggedDate = time.Now().Add(24 * time.Hour).Truncate(time.Second)

	err = repo.SaveStreak(ctx, retrieved)
	assert.NoError(t, err)

	updated, err := repo.GetStreak(ctx, userID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, retrieved.ID, updated.ID)
	assert.Equal(t, 4, updated.CurrentStreak)
	assert.Equal(t, 6, updated.MaxStreak)
	assert.True(t, retrieved.LastLoggedDate.Equal(updated.LastLoggedDate))
}

