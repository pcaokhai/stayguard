package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	auditCheckInEdited = "stay.check_in_edited"
	auditMoved         = "stay.moved"
	editIDPrefix       = "sd"
	editKindCheckIn    = "CHECK_IN"
	editKindMove       = "MOVE"
)

// EditCheckInInput and MoveStayInput are the raw request bodies.
type EditCheckInInput struct {
	NewCheckInAt     time.Time
	ReasonCode, Note string
}

type MoveStayInput struct{ ToRoomID, RentalType string }

// StayEdits holds the corrections of an active stay: check-in time and move to another room (SG-801).
type StayEdits struct {
	uow    UnitOfWork
	repo   StayEditRepo
	enc    Encryptor
	idem   IdempotencyStore
	audit  AuditWriter
	alerts AlertWriter
	ids    IDGenerator
	clock  Clock
	guard
}

func NewStayEdits(uow UnitOfWork, repo StayEditRepo, levels BuildingLevels, enc Encryptor, idem IdempotencyStore,
	audit AuditWriter, alerts AlertWriter, ids IDGenerator, clock Clock) *StayEdits {
	return &StayEdits{uow: uow, repo: repo, enc: enc, idem: idem, audit: audit, alerts: alerts, ids: ids, clock: clock,
		guard: guard{levels: levels}}
}

// EditCheckIn corrects the check-in time of an ACTIVE stay (docs/15 rule 3) and raises STAY_TIME_EDITED.
// The price follows from the new time through domain/pricing when the stay is read or checked out.
func (s *StayEdits) EditCheckIn(ctx context.Context, c Caller, stayID, retryID string, in EditCheckInInput) (StayDetail, error) {
	const op = "editCheckInTime"
	if err := s.checkRole(op, c); err != nil {
		return StayDetail{}, err
	}
	if err := checkKey(retryID); err != nil {
		return StayDetail{}, err
	}
	hash := editHash(in)
	return s.run(ctx, c, op, stayID, "POST /v1/stays/"+stayID+"/check-in-time", retryID, hash,
		func(ctx context.Context, tx Tx, rec StayRecord) (StayRecord, error) {
			return s.applyCheckIn(ctx, tx, c, rec, in)
		})
}

// MoveStay moves an ACTIVE stay to another vacant room: check-in time and extras stay, the whole stay is
// priced with the new room type's rate plan, and the old room becomes TO_CLEAN (docs/15 rule 4).
func (s *StayEdits) MoveStay(ctx context.Context, c Caller, stayID, retryID string, in MoveStayInput) (StayDetail, error) {
	const op = "moveStay"
	if err := s.checkRole(op, c); err != nil {
		return StayDetail{}, err
	}
	if err := checkKey(retryID); err != nil {
		return StayDetail{}, err
	}
	rental, err := pricing.ParseRentalType(in.RentalType)
	if err != nil { // the parser echoes the value; keep it out of the error
		return StayDetail{}, pricing.ErrUnknownRentalType
	}
	hash := RequestHash([]byte(fmt.Sprintf(`{"toRoomId":%q,"rentalType":%q}`, in.ToRoomID, in.RentalType)))
	return s.run(ctx, c, op, stayID, "POST /v1/stays/"+stayID+"/move", retryID, hash,
		func(ctx context.Context, tx Tx, rec StayRecord) (StayRecord, error) {
			return s.applyMove(ctx, tx, c, rec, in.ToRoomID, rental)
		})
}

func editHash(in EditCheckInInput) string {
	b, _ := json.Marshal(struct {
		At     string `json:"newCheckInAt"`
		Reason string `json:"reasonCode"`
		Note   string `json:"note"`
	}{in.NewCheckInAt.UTC().Format(time.RFC3339Nano), in.ReasonCode, in.Note}) // plain strings: cannot fail
	return RequestHash(b)
}

// run is the shared frame: lock the stay (a foreign id is a 404 before a 403), check the building, claim the
// idempotency key, require ACTIVE, apply, answer with the stay as stored and keep the answer under the key.
func (s *StayEdits) run(ctx context.Context, c Caller, op, stayID, route, key, hash string,
	apply func(context.Context, Tx, StayRecord) (StayRecord, error)) (StayDetail, error) {
	var out StayDetail
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, ok, err := s.repo.LockStay(ctx, tx, stayID)
		if err != nil {
			return fmt.Errorf("lock stay: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := s.checkBuilding(ctx, op, c, rec.BuildingID); err != nil {
			return err
		}
		oc, err := s.idem.Begin(ctx, tx, route, key, hash)
		if err != nil {
			return err
		}
		if oc.Replay {
			return json.Unmarshal(oc.Body, &out)
		}
		if st, err := stay.ParseStatus(rec.Status); err != nil {
			return fmt.Errorf("stay status: %w", err)
		} else if st != stay.StatusActive {
			return stay.ErrNotActive
		}
		updated, err := apply(ctx, tx, rec)
		if err != nil {
			return err
		}
		if out, err = s.detail(ctx, tx, c, updated); err != nil {
			return err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		return s.idem.Complete(ctx, tx, route, key, statusOK, body)
	})
	return out, err
}

func (s *StayEdits) applyCheckIn(ctx context.Context, tx Tx, c Caller, rec StayRecord, in EditCheckInInput) (StayRecord, error) {
	now := storedTime(s.clock.Now())
	original, ok, err := s.repo.FirstRecordedCheckIn(ctx, tx, rec.ID)
	if err != nil {
		return StayRecord{}, fmt.Errorf("first check-in: %w", err)
	}
	if !ok {
		original = rec.CheckInAt
	}
	newAt := storedTime(in.NewCheckInAt)
	if err := stay.ValidateCheckInEdit(original, newAt, now, in.ReasonCode, in.Note); err != nil {
		return StayRecord{}, err
	}
	if newAt.Equal(rec.CheckInAt) {
		return StayRecord{}, stay.NewValidationError([]stay.FieldError{{Path: "newCheckInAt", Code: "UNCHANGED"}})
	}
	if err := s.repo.UpdateCheckIn(ctx, tx, rec.ID, newAt); err != nil {
		return StayRecord{}, fmt.Errorf("update check-in: %w", err)
	}
	old := rec.CheckInAt
	edit := StayEdit{ID: s.ids.New(editIDPrefix), StayID: rec.ID, Kind: editKindCheckIn, ActorID: c.UserID,
		OldCheckInAt: &old, NewCheckInAt: &newAt, ReasonCode: in.ReasonCode, Note: in.Note, CreatedAt: now}
	if err := s.repo.InsertEdit(ctx, tx, edit); err != nil {
		return StayRecord{}, fmt.Errorf("insert edit: %w", err)
	}
	alert := AlertDraft{ID: s.ids.New(alertIDPrefix), Kind: AlertStayTimeEdited, RoomCode: rec.RoomCode, StayID: rec.ID, By: c.UserID,
		Details: map[string]string{"oldTime": rec.CheckInAt.UTC().Format(time.RFC3339), "newTime": newAt.Format(time.RFC3339),
			"reasonCode": in.ReasonCode}}
	if err := s.alerts.Raise(ctx, tx, alert); err != nil {
		return StayRecord{}, fmt.Errorf("raise alert: %w", err)
	}
	after := map[string]any{"stayId": rec.ID, "oldCheckInAt": rec.CheckInAt.UTC(), "newCheckInAt": newAt, "reasonCode": in.ReasonCode}
	if err := s.auditAppend(ctx, tx, c, auditCheckInEdited, rec.ID, after); err != nil {
		return StayRecord{}, err
	}
	rec.CheckInAt = newAt
	return rec, nil
}

func (s *StayEdits) applyMove(ctx context.Context, tx Tx, c Caller, rec StayRecord, toRoomID string, rental pricing.RentalType) (StayRecord, error) {
	if toRoomID == rec.RoomID {
		return StayRecord{}, stay.NewValidationError([]stay.FieldError{{Path: "toRoomId", Code: "SAME_ROOM"}})
	}
	from, to, err := s.lockRooms(ctx, tx, rec.RoomID, toRoomID)
	if err != nil {
		return StayRecord{}, err
	}
	if err := s.checkBuilding(ctx, "moveStay", c, to.BuildingID); err != nil {
		return StayRecord{}, err
	}
	stored, err := room.ParseStored(to.StoredStatus)
	if err != nil {
		return StayRecord{}, err
	}
	if _, err := room.CheckIn(stored); err != nil {
		return StayRecord{}, err
	}
	plan, err := pricing.ParseRatePlan(to.RatePlan)
	if err != nil {
		return StayRecord{}, fmt.Errorf("target rate plan: %w", err)
	}
	moved := rec
	moved.RoomID, moved.RoomCode, moved.BuildingID = to.ID, to.Code, to.BuildingID
	moved.RentalType, moved.RatePlanSnapshot = string(rental), plan.Snapshot()
	if err := s.checkPriced(ctx, tx, moved); err != nil {
		return StayRecord{}, err
	}
	if _, err := room.MoveOut(room.Status(from.StoredStatus)); err != nil {
		return StayRecord{}, err
	}
	m := MovedStay{StayID: rec.ID, ToRoomID: to.ID, RentalType: string(rental), RatePlanSnapshot: moved.RatePlanSnapshot,
		RatePlanSchema: rateSnapshotSchema}
	if err := s.repo.MoveStay(ctx, tx, m); err != nil {
		return StayRecord{}, fmt.Errorf("move stay: %w", err)
	}
	if err := s.repo.MarkRoomOccupied(ctx, tx, to.ID); err != nil {
		return StayRecord{}, fmt.Errorf("occupy room: %w", err)
	}
	if err := s.repo.MarkRoomToClean(ctx, tx, rec.RoomID); err != nil {
		return StayRecord{}, fmt.Errorf("release room: %w", err)
	}
	edit := StayEdit{ID: s.ids.New(editIDPrefix), StayID: rec.ID, Kind: editKindMove, ActorID: c.UserID,
		FromRoomCode: rec.RoomCode, ToRoomCode: to.Code, CreatedAt: storedTime(s.clock.Now())}
	if err := s.repo.InsertEdit(ctx, tx, edit); err != nil {
		return StayRecord{}, fmt.Errorf("insert edit: %w", err)
	}
	after := map[string]any{"stayId": rec.ID, "fromRoomId": rec.RoomID, "toRoomId": to.ID, "rentalType": rental,
		"pricingVersion": plan.Version}
	if err := s.auditAppend(ctx, tx, c, auditMoved, rec.ID, after); err != nil {
		return StayRecord{}, err
	}
	return moved, nil
}

// lockRooms takes both room locks in id order, so two opposite moves cannot deadlock. A missing target is a 404.
func (s *StayEdits) lockRooms(ctx context.Context, tx Tx, fromID, toID string) (from, to CheckInRoom, err error) {
	ids := []string{fromID, toID}
	if toID < fromID {
		ids = []string{toID, fromID}
	}
	rooms := map[string]CheckInRoom{}
	for _, id := range ids {
		r, ok, err := s.repo.Room(ctx, tx, id, true)
		if err != nil {
			return CheckInRoom{}, CheckInRoom{}, fmt.Errorf("lock room: %w", err)
		}
		if !ok {
			return CheckInRoom{}, CheckInRoom{}, ErrNotFound
		}
		rooms[id] = r
	}
	return rooms[fromID], rooms[toID], nil
}

// checkPriced proves the new plan prices the stay (the room type may not offer the rental type) before anything is written.
func (s *StayEdits) checkPriced(ctx context.Context, tx Tx, rec StayRecord) error {
	loc, err := loadZone(ctx, tx, s.repo)
	if err != nil {
		return err
	}
	_, err = detailFor(rec, storedTime(s.clock.Now()), nil, loc)
	return err
}

func (s *StayEdits) detail(ctx context.Context, tx Tx, c Caller, rec StayRecord) (StayDetail, error) {
	loc, err := loadZone(ctx, tx, s.repo)
	if err != nil {
		return StayDetail{}, err
	}
	masked, err := maskedStored(s.enc, c.TenantID, rec)
	if err != nil {
		return StayDetail{}, err
	}
	asOf, err := quoteInstant(rec, s.clock.Now())
	if err != nil {
		return StayDetail{}, err
	}
	return detailFor(rec, asOf, masked, loc)
}

func (s *StayEdits) auditAppend(ctx context.Context, tx Tx, c Caller, action, stayID string, after any) error {
	raw, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: s.ids.New(auditIDPrefix), ActorID: c.UserID, Action: action, EntityType: entityStay, EntityID: stayID, After: raw}
	if err := s.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}
