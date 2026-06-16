package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
	"gorm.io/gorm"
)

// FamilyLinkEntity GORM persistence representation of FamilyLink
type FamilyLinkEntity struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	CreatedAt  time.Time `gorm:"not null"`
	SponsorID  uint      `gorm:"not null;uniqueIndex:idx_sponsor_subject"`
	SubjectID  uint      `gorm:"not null;uniqueIndex:idx_sponsor_subject"`
	Relation   string    `gorm:"not null"`
	AccessRole string    `gorm:"not null"`
}

// TableName overrides the table name for FamilyLinkEntity
func (FamilyLinkEntity) TableName() string {
	return "family_links"
}

// VitalTelemetryEntity GORM persistence representation of VitalTelemetry
type VitalTelemetryEntity struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	SubjectID   uint      `gorm:"not null;index"`
	VitalType   string    `gorm:"not null;index"`
	ValueMetric float64   `gorm:"not null"`
	ValueUnit   string    `gorm:"not null"`
	ContextData string    `gorm:"type:text"` // Encrypted PHI field
	RecordedAt  time.Time `gorm:"not null;index"`
}

// TableName overrides the table name for VitalTelemetryEntity
func (VitalTelemetryEntity) TableName() string {
	return "vital_telemetries"
}

// EmergencyQREntity GORM persistence representation of EmergencyQR
type EmergencyQREntity struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	SubjectID    uint   `gorm:"not null;uniqueIndex"`
	QRHash       string `gorm:"not null;uniqueIndex"`
	BloodGroup   string `gorm:"not null"`
	Allergies    string `gorm:"type:text"`
	ActiveMeds   string `gorm:"type:text"` // Encrypted PHI field
	SponsorPhone string `gorm:"not null"`
	IsActive     bool   `gorm:"not null;default:true"`
}

// TableName overrides the table name for EmergencyQREntity
func (EmergencyQREntity) TableName() string {
	return "emergency_qrs"
}

// UserStreakEntity GORM persistence representation of UserStreak
type UserStreakEntity struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	UserID         uint      `gorm:"not null;uniqueIndex"`
	CurrentStreak  int       `gorm:"not null;default:0"`
	MaxStreak      int       `gorm:"not null;default:0"`
	LastLoggedDate time.Time `gorm:"not null"`
}

// TableName overrides the table name for UserStreakEntity
func (UserStreakEntity) TableName() string {
	return "user_streaks"
}

// GormRepository implements repository.FamilyRepository, repository.TelemetryRepository,
// and repository.EmergencyRepository.
type GormRepository struct {
	db     *gorm.DB
	crypto *security.CryptoEngine
}

// Ensure interface compliance
var _ repository.FamilyRepository = (*GormRepository)(nil)
var _ repository.TelemetryRepository = (*GormRepository)(nil)
var _ repository.EmergencyRepository = (*GormRepository)(nil)
var _ repository.StreakRepository = (*GormRepository)(nil)

// NewGormRepository creates a new instance of GormRepository
func NewGormRepository(db *gorm.DB, crypto *security.CryptoEngine) *GormRepository {
	return &GormRepository{
		db:     db,
		crypto: crypto,
	}
}

// Migrate auto-migrates the database schemas for persistence entities
func (r *GormRepository) Migrate() error {
	if r.db.Dialector.Name() == "sqlite" {
		sqls := []string{
			`CREATE TABLE IF NOT EXISTS family_links (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at DATETIME NOT NULL,
				sponsor_id INTEGER NOT NULL,
				subject_id INTEGER NOT NULL,
				relation TEXT NOT NULL,
				access_role TEXT NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_sponsor_subject ON family_links(sponsor_id, subject_id);`,

			`CREATE TABLE IF NOT EXISTS vital_telemetries (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				subject_id INTEGER NOT NULL,
				vital_type TEXT NOT NULL,
				value_metric REAL NOT NULL,
				value_unit TEXT NOT NULL,
				context_data TEXT,
				recorded_at DATETIME NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_vital_telemetries_subject_id ON vital_telemetries(subject_id);`,
			`CREATE INDEX IF NOT EXISTS idx_vital_telemetries_vital_type ON vital_telemetries(vital_type);`,
			`CREATE INDEX IF NOT EXISTS idx_vital_telemetries_recorded_at ON vital_telemetries(recorded_at);`,

			`CREATE TABLE IF NOT EXISTS emergency_qrs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				subject_id INTEGER NOT NULL,
				qr_hash TEXT NOT NULL,
				blood_group TEXT NOT NULL,
				allergies TEXT,
				active_meds TEXT,
				sponsor_phone TEXT NOT NULL,
				is_active BOOLEAN NOT NULL DEFAULT 1
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_emergency_qrs_subject_id ON emergency_qrs(subject_id);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_emergency_qrs_qr_hash ON emergency_qrs(qr_hash);`,

			`CREATE TABLE IF NOT EXISTS user_streaks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				current_streak INTEGER NOT NULL DEFAULT 0,
				max_streak INTEGER NOT NULL DEFAULT 0,
				last_logged_date DATETIME NOT NULL
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_streaks_user_id ON user_streaks(user_id);`,
		}

		for _, sql := range sqls {
			if err := r.db.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to run SQLite migration: %w", err)
			}
		}
		return nil
	}

	return r.db.AutoMigrate(
		&FamilyLinkEntity{},
		&VitalTelemetryEntity{},
		&EmergencyQREntity{},
		&UserStreakEntity{},
	)
}

// ============================================================================
// FamilyRepository Implementation
// ============================================================================

// CreateLink inserts a new FamilyLink record into the database
func (r *GormRepository) CreateLink(ctx context.Context, link *model.FamilyLink) error {
	if link == nil {
		return errors.New("family link cannot be nil")
	}
	entity := toFamilyLinkEntity(link)
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return err
	}
	link.ID = entity.ID
	link.CreatedAt = entity.CreatedAt
	return nil
}

// GetLink retrieves a FamilyLink for a given sponsor and subject
func (r *GormRepository) GetLink(ctx context.Context, sponsorID, subjectID uint) (*model.FamilyLink, error) {
	var entity FamilyLinkEntity
	err := r.db.WithContext(ctx).
		Where("sponsor_id = ? AND subject_id = ?", sponsorID, subjectID).
		First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toFamilyLinkDomain(&entity), nil
}

// ListLinksBySponsor lists all family links where the user is the sponsor
func (r *GormRepository) ListLinksBySponsor(ctx context.Context, sponsorID uint) ([]model.FamilyLink, error) {
	var entities []FamilyLinkEntity
	err := r.db.WithContext(ctx).
		Where("sponsor_id = ?", sponsorID).
		Find(&entities).Error
	if err != nil {
		return nil, err
	}

	links := make([]model.FamilyLink, len(entities))
	for i, entity := range entities {
		links[i] = *toFamilyLinkDomain(&entity)
	}
	return links, nil
}

// DeleteLink removes a family link relationship from the database
func (r *GormRepository) DeleteLink(ctx context.Context, sponsorID, subjectID uint) error {
	res := r.db.WithContext(ctx).
		Where("sponsor_id = ? AND subject_id = ?", sponsorID, subjectID).
		Delete(&FamilyLinkEntity{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ============================================================================
// TelemetryRepository Implementation
// ============================================================================

// RecordVitals encrypts context data and records a new vital telemetry reading
func (r *GormRepository) RecordVitals(ctx context.Context, vitals *model.VitalTelemetry) error {
	if vitals == nil {
		return errors.New("vitals cannot be nil")
	}
	if vitals.RecordedAt.IsZero() {
		vitals.RecordedAt = time.Now()
	}

	entity, err := r.toTelemetryEntity(vitals)
	if err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return err
	}

	vitals.ID = entity.ID
	return nil
}

// ListVitals retrieves the most recent vital telemetry records for a subject, with filtering and limits
func (r *GormRepository) ListVitals(ctx context.Context, subjectID uint, vitalType string, limit int) ([]model.VitalTelemetry, error) {
	var entities []VitalTelemetryEntity
	query := r.db.WithContext(ctx).Where("subject_id = ?", subjectID)
	if vitalType != "" {
		query = query.Where("vital_type = ?", vitalType)
	}
	query = query.Order("recorded_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, err
	}

	vitals := make([]model.VitalTelemetry, len(entities))
	for i, entity := range entities {
		d, err := r.toTelemetryDomain(&entity)
		if err != nil {
			return nil, err
		}
		vitals[i] = *d
	}
	return vitals, nil
}

// ============================================================================
// EmergencyRepository Implementation
// ============================================================================

// RegisterQR registers/saves an EmergencyQR record, encrypting sensitive fields before persist
func (r *GormRepository) RegisterQR(ctx context.Context, qr *model.EmergencyQR) error {
	if qr == nil {
		return errors.New("qr cannot be nil")
	}

	entity, err := r.toEmergencyEntity(qr)
	if err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return err
	}

	qr.ID = entity.ID
	return nil
}

// ResolveQR retrieves and decrypts an EmergencyQR record by its unique hash
func (r *GormRepository) ResolveQR(ctx context.Context, qrHash string) (*model.EmergencyQR, error) {
	var entity EmergencyQREntity
	err := r.db.WithContext(ctx).
		Where("qr_hash = ?", qrHash).
		First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return r.toEmergencyDomain(&entity)
}

// ============================================================================
// Mappers and Helpers
// ============================================================================

func toFamilyLinkEntity(d *model.FamilyLink) *FamilyLinkEntity {
	if d == nil {
		return nil
	}
	return &FamilyLinkEntity{
		ID:         d.ID,
		CreatedAt:  d.CreatedAt,
		SponsorID:  d.SponsorID,
		SubjectID:  d.SubjectID,
		Relation:   d.Relation,
		AccessRole: d.AccessRole,
	}
}

func toFamilyLinkDomain(e *FamilyLinkEntity) *model.FamilyLink {
	if e == nil {
		return nil
	}
	return &model.FamilyLink{
		ID:         e.ID,
		CreatedAt:  e.CreatedAt,
		SponsorID:  e.SponsorID,
		SubjectID:  e.SubjectID,
		Relation:   e.Relation,
		AccessRole: e.AccessRole,
	}
}

func (r *GormRepository) toTelemetryEntity(d *model.VitalTelemetry) (*VitalTelemetryEntity, error) {
	if d == nil {
		return nil, nil
	}
	var encryptedContext string
	if d.ContextData != "" {
		var err error
		encryptedContext, err = r.crypto.Encrypt(d.ContextData)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt context data: %w", err)
		}
	}
	return &VitalTelemetryEntity{
		ID:          d.ID,
		SubjectID:   d.SubjectID,
		VitalType:   d.VitalType,
		ValueMetric: d.ValueMetric,
		ValueUnit:   d.ValueUnit,
		ContextData: encryptedContext,
		RecordedAt:  d.RecordedAt,
	}, nil
}

func (r *GormRepository) toTelemetryDomain(e *VitalTelemetryEntity) (*model.VitalTelemetry, error) {
	if e == nil {
		return nil, nil
	}
	var decryptedContext string
	if e.ContextData != "" {
		var err error
		decryptedContext, err = r.crypto.Decrypt(e.ContextData)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt context data: %w", err)
		}
	}
	return &model.VitalTelemetry{
		ID:          e.ID,
		SubjectID:   e.SubjectID,
		VitalType:   e.VitalType,
		ValueMetric: e.ValueMetric,
		ValueUnit:   e.ValueUnit,
		ContextData: decryptedContext,
		RecordedAt:  e.RecordedAt,
	}, nil
}

func (r *GormRepository) toEmergencyEntity(d *model.EmergencyQR) (*EmergencyQREntity, error) {
	if d == nil {
		return nil, nil
	}
	var encryptedMeds string
	if d.ActiveMeds != "" {
		var err error
		encryptedMeds, err = r.crypto.Encrypt(d.ActiveMeds)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt active meds: %w", err)
		}
	}
	return &EmergencyQREntity{
		ID:           d.ID,
		SubjectID:    d.SubjectID,
		QRHash:       d.QRHash,
		BloodGroup:   d.BloodGroup,
		Allergies:    d.Allergies,
		ActiveMeds:   encryptedMeds,
		SponsorPhone: d.SponsorPhone,
		IsActive:     d.IsActive,
	}, nil
}

func (r *GormRepository) toEmergencyDomain(e *EmergencyQREntity) (*model.EmergencyQR, error) {
	if e == nil {
		return nil, nil
	}
	var decryptedMeds string
	if e.ActiveMeds != "" {
		var err error
		decryptedMeds, err = r.crypto.Decrypt(e.ActiveMeds)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt active meds: %w", err)
		}
	}
	return &model.EmergencyQR{
		ID:           e.ID,
		SubjectID:    e.SubjectID,
		QRHash:       e.QRHash,
		BloodGroup:   e.BloodGroup,
		Allergies:    e.Allergies,
		ActiveMeds:   decryptedMeds,
		SponsorPhone: e.SponsorPhone,
		IsActive:     e.IsActive,
	}, nil
}

// ============================================================================
// StreakRepository Implementation
// ============================================================================

// GetStreak retrieves a UserStreak for a user ID
func (r *GormRepository) GetStreak(ctx context.Context, userID uint) (*model.UserStreak, error) {
	var entity UserStreakEntity
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toUserStreakDomain(&entity), nil
}

// SaveStreak saves/upserts a UserStreak record
func (r *GormRepository) SaveStreak(ctx context.Context, streak *model.UserStreak) error {
	if streak == nil {
		return errors.New("streak cannot be nil")
	}
	entity := toUserStreakEntity(streak)

	var existing UserStreakEntity
	err := r.db.WithContext(ctx).Where("user_id = ?", entity.UserID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
				return err
			}
			streak.ID = entity.ID
			return nil
		}
		return err
	}

	entity.ID = existing.ID
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return err
	}
	streak.ID = entity.ID
	return nil
}

func toUserStreakEntity(d *model.UserStreak) *UserStreakEntity {
	if d == nil {
		return nil
	}
	return &UserStreakEntity{
		ID:             d.ID,
		UserID:         d.UserID,
		CurrentStreak:  d.CurrentStreak,
		MaxStreak:      d.MaxStreak,
		LastLoggedDate: d.LastLoggedDate,
	}
}

func toUserStreakDomain(e *UserStreakEntity) *model.UserStreak {
	if e == nil {
		return nil
	}
	return &model.UserStreak{
		ID:             e.ID,
		UserID:         e.UserID,
		CurrentStreak:  e.CurrentStreak,
		MaxStreak:      e.MaxStreak,
		LastLoggedDate: e.LastLoggedDate,
	}
}
