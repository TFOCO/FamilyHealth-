package family

import (
	"context"
	"errors"

	"github.com/fastenhealth/fasten-onprem/backend/domain/model"
	"github.com/fastenhealth/fasten-onprem/backend/domain/repository"
	"github.com/fastenhealth/fasten-onprem/backend/infrastructure/security"
)

// FamilyUseCase defines the orchestrator interface for family-related business logic.
type FamilyUseCase interface {
	LinkFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint, relation, accessRole string) error
	CheckRelationship(ctx context.Context, operatorID uint, sponsorID, subjectID uint) (bool, error)
	ListFamilyLinks(ctx context.Context, operatorID uint, sponsorID uint) ([]model.FamilyLink, error)
	RemoveFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint) error
}

type familyUseCase struct {
	familyRepo  repository.FamilyRepository
	auditLogger *security.AuditLogger
}

// NewFamilyUseCase creates a new instance of FamilyUseCase.
func NewFamilyUseCase(familyRepo repository.FamilyRepository, auditLogger *security.AuditLogger) FamilyUseCase {
	return &familyUseCase{
		familyRepo:  familyRepo,
		auditLogger: auditLogger,
	}
}

// LinkFamilyMember links a sponsor to a subject (family member) and logs a WRITE_PHI audit record.
func (u *familyUseCase) LinkFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint, relation, accessRole string) error {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionWritePHI, subjectID, "FamilyLink", "usecase", "backend")
	if err != nil {
		return err
	}

	// Verify if link already exists
	existing, err := u.familyRepo.GetLink(ctx, sponsorID, subjectID)
	if err == nil && existing != nil {
		return errors.New("family link already exists")
	}

	link := &model.FamilyLink{
		SponsorID:  sponsorID,
		SubjectID:  subjectID,
		Relation:   relation,
		AccessRole: accessRole,
	}

	return u.familyRepo.CreateLink(ctx, link)
}

// CheckRelationship verifies if a sponsor has a valid relationship with a subject and logs a READ_PHI audit record.
func (u *familyUseCase) CheckRelationship(ctx context.Context, operatorID uint, sponsorID, subjectID uint) (bool, error) {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, subjectID, "FamilyLink", "usecase", "backend")
	if err != nil {
		return false, err
	}

	link, err := u.familyRepo.GetLink(ctx, sponsorID, subjectID)
	if err != nil {
		return false, err
	}
	if link == nil {
		return false, nil
	}

	return true, nil
}

// ListFamilyLinks lists all linked family members for a sponsor and logs a READ_PHI audit record for the sponsor's relationships.
func (u *familyUseCase) ListFamilyLinks(ctx context.Context, operatorID uint, sponsorID uint) ([]model.FamilyLink, error) {
	// HIPAA Compliance: Log access event for reading the relationship metadata of the sponsor
	err := u.auditLogger.Log(operatorID, security.ActionReadPHI, sponsorID, "FamilyLink", "usecase", "backend")
	if err != nil {
		return nil, err
	}

	return u.familyRepo.ListLinksBySponsor(ctx, sponsorID)
}

// RemoveFamilyMember deletes the relationship between a sponsor and a subject and logs a DELETE_PHI audit record.
func (u *familyUseCase) RemoveFamilyMember(ctx context.Context, operatorID uint, sponsorID, subjectID uint) error {
	// HIPAA Compliance: Log access event
	err := u.auditLogger.Log(operatorID, security.ActionDeletePHI, subjectID, "FamilyLink", "usecase", "backend")
	if err != nil {
		return err
	}

	return u.familyRepo.DeleteLink(ctx, sponsorID, subjectID)
}
