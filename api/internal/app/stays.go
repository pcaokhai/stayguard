package app

import (
	"context"
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	// idNumberField names the encrypted column; the stay id is appended so a ciphertext copied
	// to another stay's row does not open.
	idNumberField = "stays.id_number"
	stayIDPrefix  = "st"
	auditIDPrefix = "au"
)

// Stays holds the check-in use cases (SG-203). It does no I/O of its own: everything goes through ports.
type Stays struct {
	uow    UnitOfWork
	repo   StayRepo
	levels BuildingLevels
	enc    Encryptor
	idem   IdempotencyStore
	audit  AuditWriter
	ids    IDGenerator
	clock  Clock
	authz  access.Authorizer
}

func NewStays(uow UnitOfWork, repo StayRepo, levels BuildingLevels, enc Encryptor, idem IdempotencyStore,
	audit AuditWriter, ids IDGenerator, clock Clock) *Stays {
	return &Stays{uow: uow, repo: repo, levels: levels, enc: enc, idem: idem, audit: audit, ids: ids, clock: clock}
}

// checkRole applies only the role half of the rule: EDIT satisfies any building level.
func (s *Stays) checkRole(op string, c Caller) error { return s.authz.Check(op, c.Role, access.EDIT) }

// checkBuilding runs after the existence check, so a foreign id is a 404 before it can be a 403.
func (s *Stays) checkBuilding(ctx context.Context, op string, c Caller, buildingID string) error {
	lv, err := s.levels.Levels(ctx, c, []string{buildingID})
	if err != nil {
		return fmt.Errorf("building levels: %w", err)
	}
	return s.authz.Check(op, c.Role, lv[buildingID]) // missing id is NONE
}

// idField is the encryption field of one stay's ID number.
func idField(stayID string) string { return idNumberField + "/" + stayID }
