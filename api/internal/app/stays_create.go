package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	// statusCreated is stored with the replayable response (app cannot import net/http).
	statusCreated = 201
	// rateSnapshotSchema is the version of the snapshot encoding stored with each stay (plan Ruling 3).
	rateSnapshotSchema = 1
	// maxIdempotencyKeyBytes bounds the key before it reaches the store (a UUID is 36).
	maxIdempotencyKeyBytes = 64
	auditCheckIn           = "stay.check_in"
	entityStay             = "stay"
)

// ErrInvalidIdempotencyKey: the key is empty or longer than maxIdempotencyKeyBytes (HTTP 422). It never carries the key.
var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")

// CreateStayInput is the raw request body; IDNumber is nil when the guest gave none.
type CreateStayInput struct {
	RentalType, GuestName, GuestPhone string
	IDNumber                          *string
	Deposit                           int64
}

// checkInRequest is what a valid input becomes: trimmed guest values, parsed rental type, deposit.
type checkInRequest struct {
	rental  pricing.RentalType
	guest   stay.Guest
	deposit int64
}

// stayRoute includes the room id so one key cannot replay another room's stay (plan Ruling 6).
func stayRoute(roomID string) string { return "POST /v1/rooms/" + roomID + "/stays" }

func validateCreate(in CreateStayInput) (checkInRequest, error) {
	id := ""
	if in.IDNumber != nil {
		id = *in.IDNumber
	}
	g, err := stay.ValidateGuest(in.GuestName, in.GuestPhone, id)
	if err != nil {
		return checkInRequest{}, fmt.Errorf("check-in input: %w", err)
	}
	d, err := stay.ValidateDeposit(in.Deposit)
	if err != nil {
		return checkInRequest{}, fmt.Errorf("check-in input: %w", err)
	}
	rental, err := pricing.ParseRentalType(in.RentalType)
	if err != nil { // the parser echoes the value; keep it out of the error
		return checkInRequest{}, fmt.Errorf("check-in input: %w", pricing.ErrUnknownRentalType)
	}
	return checkInRequest{rental: rental, guest: g, deposit: d.Int64()}, nil
}

// CreateStay checks a guest in. The order is fixed: role, input, room and level, idempotency key,
// then the locked write. replayed is true when the stored response of an earlier identical call is returned.
func (s *Stays) CreateStay(ctx context.Context, c Caller, roomID, retryID string, in CreateStayInput) (StayDetail, bool, error) {
	const op = "createStay"
	if err := s.checkRole(op, c); err != nil {
		return StayDetail{}, false, err
	}
	if retryID == "" || len(retryID) > maxIdempotencyKeyBytes {
		return StayDetail{}, false, ErrInvalidIdempotencyKey
	}
	req, err := validateCreate(in)
	if err != nil {
		return StayDetail{}, false, err
	}
	var out StayDetail
	var replayed bool
	err = s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		out, replayed, err = s.checkIn(ctx, tx, c, roomID, retryID, in, req)
		return err
	})
	return out, replayed, err
}

func (s *Stays) checkIn(ctx context.Context, tx Tx, c Caller, roomID, key string, in CreateStayInput, req checkInRequest) (StayDetail, bool, error) {
	r, ok, err := s.repo.Room(ctx, tx, roomID, false)
	if err != nil {
		return StayDetail{}, false, fmt.Errorf("room: %w", err)
	}
	if !ok {
		return StayDetail{}, false, ErrNotFound
	}
	if err := s.checkBuilding(ctx, "createStay", c, r.BuildingID); err != nil {
		return StayDetail{}, false, err
	}
	route := stayRoute(roomID)
	oc, err := s.idem.Begin(ctx, tx, route, key, s.requestHash(c.TenantID, in))
	if err != nil {
		return StayDetail{}, false, err
	}
	if oc.Replay {
		var v StayDetail
		if err := json.Unmarshal(oc.Body, &v); err != nil {
			return StayDetail{}, false, fmt.Errorf("stored stay response: %w", err)
		}
		return v, true, nil
	}
	v, err := s.insertStay(ctx, tx, c, roomID, in, req)
	if err != nil {
		return StayDetail{}, false, err
	}
	body, err := json.Marshal(v)
	if err != nil {
		return StayDetail{}, false, fmt.Errorf("encode stay response: %w", err)
	}
	if err := s.idem.Complete(ctx, tx, route, key, statusCreated, body); err != nil {
		return StayDetail{}, false, err
	}
	return v, false, nil
}

// requestHash uses the raw, untrimmed input so "different body" is exact, and a keyed fingerprint
// of the ID number so the idempotency table holds nothing that can be brute-forced (plan Ruling 9).
func (s *Stays) requestHash(tenantID string, in CreateStayInput) string {
	fp := ""
	// Trimmed like the stored value: null, "" and "   " all mean no id number, so a retry that
	// switches between them replays instead of conflicting. Other fields hash raw.
	if in.IDNumber != nil && strings.TrimSpace(*in.IDNumber) != "" {
		fp = hex.EncodeToString(s.enc.Fingerprint(tenantID, idNumberField, []byte(strings.TrimSpace(*in.IDNumber))))
	}
	body, _ := json.Marshal(struct {
		RentalType    string `json:"rentalType"`
		GuestName     string `json:"guestName"`
		GuestPhone    string `json:"guestPhone"`
		IDFingerprint string `json:"idNumberFingerprint"`
		Deposit       int64  `json:"deposit"`
	}{in.RentalType, in.GuestName, in.GuestPhone, fp, in.Deposit}) // plain fields only: cannot fail
	return RequestHash(body)
}

// insertStay runs under the room row lock: re-reads the room, requires VACANT and writes the effect.
func (s *Stays) insertStay(ctx context.Context, tx Tx, c Caller, roomID string, in CreateStayInput, req checkInRequest) (StayDetail, error) {
	r, ok, err := s.repo.Room(ctx, tx, roomID, true)
	if err != nil {
		return StayDetail{}, fmt.Errorf("lock room: %w", err)
	}
	if !ok {
		return StayDetail{}, ErrNotFound
	}
	stored, err := room.ParseStored(r.StoredStatus)
	if err != nil {
		return StayDetail{}, err
	}
	if _, err := room.CheckIn(stored); err != nil {
		return StayDetail{}, err
	}
	plan, err := pricing.ParseRatePlan(r.RatePlan)
	if err != nil {
		return StayDetail{}, fmt.Errorf("unit type rate plan: %w", err)
	}
	ns, err := s.newStay(c, r, plan, in, req)
	if err != nil {
		return StayDetail{}, err
	}
	if err := s.persist(ctx, tx, c, ns, plan); err != nil {
		return StayDetail{}, err
	}
	if s.cash != nil {
		if err := s.cash.Record(ctx, tx, c, CashRecord{Kind: shift.Deposit, StayID: ns.ID, Amount: ns.Deposit}); err != nil {
			return StayDetail{}, fmt.Errorf("record deposit: %w", err)
		}
	}
	loc, err := s.zone(ctx, tx)
	if err != nil {
		return StayDetail{}, err
	}
	return detailFor(recordOf(ns, r), ns.CheckInAt, loc)
}

// newStay takes the time from the server clock (rule 3) and encrypts the ID number, bound to the new stay id.
func (s *Stays) newStay(c Caller, r CheckInRoom, plan pricing.RatePlan, in CreateStayInput, req checkInRequest) (NewStay, error) {
	id := s.ids.New(stayIDPrefix)
	var enc []byte
	if req.guest.IDNumber != "" {
		var err error
		if enc, err = s.enc.Encrypt(c.TenantID, idField(id), []byte(req.guest.IDNumber)); err != nil {
			return NewStay{}, fmt.Errorf("encrypt id number: %w", err)
		}
	}
	return NewStay{ID: id, RoomID: r.ID, RentalType: string(req.rental), GuestName: req.guest.Name,
		GuestPhone: req.guest.Phone, IDNumberEnc: enc, Deposit: req.deposit, CheckInAt: storedTime(s.clock.Now()),
		RatePlanSnapshot: plan.Snapshot(), RatePlanSchema: rateSnapshotSchema, CreatedBy: c.UserID}, nil
}

func (s *Stays) persist(ctx context.Context, tx Tx, c Caller, ns NewStay, plan pricing.RatePlan) error {
	if err := s.repo.InsertStay(ctx, tx, ns); err != nil {
		return fmt.Errorf("insert stay: %w", err)
	}
	if err := s.repo.MarkRoomOccupied(ctx, tx, ns.RoomID); err != nil {
		return fmt.Errorf("occupy room: %w", err) // keeps room.ErrNotVacant visible to errors.Is
	}
	// Ids, status and amounts only: audit rows never hold names, phones or ID numbers.
	after, err := json.Marshal(map[string]any{"stayId": ns.ID, "roomId": ns.RoomID, "status": stay.StatusActive,
		"deposit": ns.Deposit, "pricingVersion": plan.Version})
	if err != nil {
		return fmt.Errorf("encode audit: %w", err)
	}
	e := AuditEntry{ID: s.ids.New(auditIDPrefix), ActorID: c.UserID, Action: auditCheckIn, EntityType: entityStay,
		EntityID: ns.ID, After: after}
	if err := s.audit.Append(ctx, tx, e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// recordOf is the stay as stored, so the create answer is built by the same code as getStay.
func recordOf(ns NewStay, r CheckInRoom) StayRecord {
	return StayRecord{ID: ns.ID, RoomID: ns.RoomID, RoomCode: r.Code, BuildingID: r.BuildingID,
		RentalType: ns.RentalType, Status: string(stay.StatusActive), GuestName: ns.GuestName,
		GuestPhone: ns.GuestPhone, GuestID: GuestIDIndicators{HasIDNumber: len(ns.IDNumberEnc) > 0}, Deposit: ns.Deposit,
		CheckInAt: ns.CheckInAt, RatePlanSnapshot: ns.RatePlanSnapshot}
}

func (s *Stays) zone(ctx context.Context, tx Tx) (*time.Location, error) {
	return loadZone(ctx, tx, s.repo)
}

// zoneSource is the slice of a stay repo that knows the tenant zone.
type zoneSource interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
}

func loadZone(ctx context.Context, tx Tx, src zoneSource) (*time.Location, error) {
	name, err := src.Timezone(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("tenant timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("tenant timezone: %w", err)
	}
	return loc, nil
}
