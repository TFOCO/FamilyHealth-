package family

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

type mockFamilyRepository struct {
	links     map[string]*model.FamilyLink
	createErr error
	getErr    error
	listErr   error
	deleteErr error
}

func newMockFamilyRepository() *mockFamilyRepository {
	return &mockFamilyRepository{
		links: make(map[string]*model.FamilyLink),
	}
}

func (m *mockFamilyRepository) CreateLink(ctx context.Context, link *model.FamilyLink) error {
	if m.createErr != nil {
		return m.createErr
	}
	key := fmt.Sprintf("%d-%d", link.SponsorID, link.SubjectID)
	m.links[key] = link
	return nil
}

func (m *mockFamilyRepository) GetLink(ctx context.Context, sponsorID, subjectID uint) (*model.FamilyLink, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	key := fmt.Sprintf("%d-%d", sponsorID, subjectID)
	link, ok := m.links[key]
	if !ok {
		return nil, nil
	}
	return link, nil
}

func (m *mockFamilyRepository) ListLinksBySponsor(ctx context.Context, sponsorID uint) ([]model.FamilyLink, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var res []model.FamilyLink
	for _, link := range m.links {
		if link.SponsorID == sponsorID {
			res = append(res, *link)
		}
	}
	return res, nil
}

func (m *mockFamilyRepository) DeleteLink(ctx context.Context, sponsorID, subjectID uint) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	key := fmt.Sprintf("%d-%d", sponsorID, subjectID)
	delete(m.links, key)
	return nil
}

func TestLinkFamilyMember(t *testing.T) {
	ctx := context.Background()
	repo := newMockFamilyRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	uc := NewFamilyUseCase(repo, logger)

	operatorID := uint(10)
	sponsorID := uint(1)
	subjectID := uint(2)

	// 1. Success case
	err := uc.LinkFamilyMember(ctx, operatorID, sponsorID, subjectID, "Father", "admin")
	assert.NoError(t, err)

	// Verify repo
	link, err := repo.GetLink(ctx, sponsorID, subjectID)
	assert.NoError(t, err)
	assert.NotNil(t, link)
	assert.Equal(t, "Father", link.Relation)
	assert.Equal(t, "admin", link.AccessRole)

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionWritePHI, record.Action)
	assert.Equal(t, subjectID, record.TargetPatientID)
	assert.Equal(t, "FamilyLink", record.ResourceType)

	// 2. Already Exists case
	auditBuf.Reset()
	err = uc.LinkFamilyMember(ctx, operatorID, sponsorID, subjectID, "Father", "admin")
	assert.Error(t, err)
	assert.Equal(t, "family link already exists", err.Error())
}

func TestCheckRelationship(t *testing.T) {
	ctx := context.Background()
	repo := newMockFamilyRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	uc := NewFamilyUseCase(repo, logger)

	operatorID := uint(10)
	sponsorID := uint(1)
	subjectID := uint(2)

	// 1. Check when relationship does not exist
	exists, err := uc.CheckRelationship(ctx, operatorID, sponsorID, subjectID)
	assert.NoError(t, err)
	assert.False(t, exists)

	// 2. Add link and check
	err = uc.LinkFamilyMember(ctx, operatorID, sponsorID, subjectID, "Father", "admin")
	assert.NoError(t, err)

	auditBuf.Reset()
	exists, err = uc.CheckRelationship(ctx, operatorID, sponsorID, subjectID)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionReadPHI, record.Action)
	assert.Equal(t, subjectID, record.TargetPatientID)
}

func TestListFamilyLinks(t *testing.T) {
	ctx := context.Background()
	repo := newMockFamilyRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	uc := NewFamilyUseCase(repo, logger)

	operatorID := uint(10)
	sponsorID := uint(1)

	// Link a few members
	_ = uc.LinkFamilyMember(ctx, operatorID, sponsorID, 2, "Father", "admin")
	_ = uc.LinkFamilyMember(ctx, operatorID, sponsorID, 3, "Mother", "viewer")

	auditBuf.Reset()
	links, err := uc.ListFamilyLinks(ctx, operatorID, sponsorID)
	assert.NoError(t, err)
	assert.Len(t, links, 2)

	// Verify Audit Log
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionReadPHI, record.Action)
	assert.Equal(t, sponsorID, record.TargetPatientID)
}

func TestRemoveFamilyMember(t *testing.T) {
	ctx := context.Background()
	repo := newMockFamilyRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	uc := NewFamilyUseCase(repo, logger)

	operatorID := uint(10)
	sponsorID := uint(1)
	subjectID := uint(2)

	// Setup link
	_ = uc.LinkFamilyMember(ctx, operatorID, sponsorID, subjectID, "Father", "admin")

	auditBuf.Reset()
	err := uc.RemoveFamilyMember(ctx, operatorID, sponsorID, subjectID)
	assert.NoError(t, err)

	// Verify deletion
	exists, err := uc.CheckRelationship(ctx, operatorID, sponsorID, subjectID)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Verify Audit Log contains Delete event
	var record security.AuditRecord
	err = json.Unmarshal(auditBuf.Bytes(), &record)
	assert.NoError(t, err)
	assert.Equal(t, operatorID, record.OperatorID)
	assert.Equal(t, security.ActionDeletePHI, record.Action)
	assert.Equal(t, subjectID, record.TargetPatientID)
}

func TestFamilyUseCaseErrors(t *testing.T) {
	ctx := context.Background()
	repo := newMockFamilyRepository()
	var auditBuf bytes.Buffer
	logger := security.NewAuditLogger(&auditBuf)
	uc := NewFamilyUseCase(repo, logger)

	// Trigger repo errors
	repo.getErr = errors.New("database connection failed")
	exists, err := uc.CheckRelationship(ctx, 10, 1, 2)
	assert.Error(t, err)
	assert.False(t, exists)
}
