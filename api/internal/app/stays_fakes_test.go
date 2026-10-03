package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const (
	bldgA    = "b1"
	roomA    = "rm1"
	idMarker = "IDMARKER12345"
)

func stayPlan() pricing.RatePlan {
	return pricing.RatePlan{
		Version: 3, Currency: pricing.CurrencyVND, GraceMinutes: 15,
		Hourly:    pricing.Hourly{FirstHour: money.Vnd(80_000), ExtraHour: 20_000},
		Overnight: pricing.Window{Price: 250_000, Start: pricing.Clock{Hour: 21}, End: pricing.Clock{Hour: 12}},
		Daily:     pricing.Window{Price: 350_000, Start: pricing.Clock{Hour: 14}, End: pricing.Clock{Hour: 12}},
	}
}

// fakeStayRepo scopes by the tenant of the tx, like the real repo.
type fakeStayRepo struct {
	rooms    map[string]map[string]CheckInRoom
	records  map[string]map[string]StayRecord
	inserted []NewStay
	tz       string
	markErr  error
	calls    int
	locks    int
	invoice  *InvoiceRecord // what InvoiceByStay returns
}

func (r *fakeStayRepo) Room(_ context.Context, tx Tx, id string, forUpdate bool) (CheckInRoom, bool, error) {
	r.calls++
	if forUpdate {
		r.locks++
	}
	rm, ok := r.rooms[tx.TenantID()][id]
	return rm, ok, nil
}

func (r *fakeStayRepo) InsertStay(_ context.Context, tx Tx, s NewStay) error {
	r.calls++
	r.inserted = append(r.inserted, s)
	rm := r.rooms[tx.TenantID()][s.RoomID]
	r.records[tx.TenantID()][s.ID] = recordOf(s, rm)
	return nil
}

func (r *fakeStayRepo) MarkRoomOccupied(_ context.Context, tx Tx, id string) error {
	r.calls++
	if r.markErr != nil {
		return r.markErr
	}
	rm := r.rooms[tx.TenantID()][id]
	rm.StoredStatus = string(room.StatusOccupied)
	r.rooms[tx.TenantID()][id] = rm
	return nil
}

func (r *fakeStayRepo) StayByID(_ context.Context, tx Tx, id string) (StayRecord, bool, error) {
	r.calls++
	rec, ok := r.records[tx.TenantID()][id]
	return rec, ok, nil
}

func (r *fakeStayRepo) Timezone(context.Context, Tx) (string, error) { r.calls++; return r.tz, nil }

type idemState struct {
	hash string
	done bool
	body []byte
	code int
}

// fakeIdem has the real semantics, scoped by (tenant, route, key).
type fakeIdem struct {
	m         map[string]idemState
	hashes    []string
	completes int
}

func (f *fakeIdem) k(tx Tx, route, key string) string { return tx.TenantID() + "|" + route + "|" + key }

func (f *fakeIdem) Begin(_ context.Context, tx Tx, route, key, hash string) (IdempotencyOutcome, error) {
	f.hashes = append(f.hashes, hash)
	st, ok := f.m[f.k(tx, route, key)]
	switch {
	case !ok:
		f.m[f.k(tx, route, key)] = idemState{hash: hash}
		return IdempotencyOutcome{}, nil
	case st.hash != hash:
		return IdempotencyOutcome{}, ErrIdempotencyKeyReused
	case !st.done:
		return IdempotencyOutcome{}, ErrIdempotencyIncomplete
	}
	return IdempotencyOutcome{Replay: true, Status: st.code, Body: st.body}, nil
}

func (f *fakeIdem) Complete(_ context.Context, tx Tx, route, key string, status int, body []byte) error {
	f.completes++
	st := f.m[f.k(tx, route, key)]
	f.m[f.k(tx, route, key)] = idemState{hash: st.hash, done: true, body: body, code: status}
	return nil
}

func (f *fakeIdem) snapshot() map[string]idemState {
	c := make(map[string]idemState, len(f.m))
	for k, v := range f.m {
		c[k] = v
	}
	return c
}

// rollbackUoW discards the idempotency keys of a failed unit of work, like a rolled back transaction.
type rollbackUoW struct {
	fakeUoW
	idem *fakeIdem
}

func (u *rollbackUoW) Do(ctx context.Context, tenantID string, fn func(context.Context, Tx) error) error {
	snap := u.idem.snapshot()
	err := u.fakeUoW.Do(ctx, tenantID, fn)
	if err != nil {
		u.idem.m = snap
	}
	return err
}

type fakeAudit struct{ entries []AuditEntry }

func (a *fakeAudit) Append(_ context.Context, _ Tx, e AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

// fakeEnc is reversible and marks ciphertext; it binds tenant and field like the real adapter.
type fakeEnc struct{ encryptErr, decryptErr error }

const encMark = "FAKECT"

func (f fakeEnc) Encrypt(tenantID, field string, plain []byte) ([]byte, error) {
	if f.encryptErr != nil {
		return nil, f.encryptErr
	}
	return []byte(encMark + "|" + tenantID + "|" + field + "|" + hex.EncodeToString(plain)), nil
}

func (f fakeEnc) Decrypt(tenantID, field string, ct []byte) ([]byte, error) {
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	prefix := encMark + "|" + tenantID + "|" + field + "|"
	if !strings.HasPrefix(string(ct), prefix) {
		return nil, errors.New("cannot decrypt")
	}
	return hex.DecodeString(strings.TrimPrefix(string(ct), prefix))
}

func (fakeEnc) Fingerprint(tenantID, field string, value []byte) []byte {
	sum := sha256.Sum256([]byte("fakekey|" + tenantID + "|" + field + "|" + string(value)))
	return sum[:]
}

type stayEnv struct {
	s      *Stays
	repo   *fakeStayRepo
	levels *fakeLevels
	idem   *fakeIdem
	audit  *fakeAudit
	uow    *rollbackUoW
	enc    fakeEnc
	caller Caller
}

func newStayEnv(t *testing.T) *stayEnv {
	t.Helper()
	plan := stayPlan()
	e := &stayEnv{
		repo: &fakeStayRepo{tz: zoneName, records: map[string]map[string]StayRecord{tenantA: {}, tenantB: {}}, rooms: map[string]map[string]CheckInRoom{
			tenantA: {roomA: {ID: roomA, Code: "101", BuildingID: bldgA, StoredStatus: string(room.StatusVacant), RatePlan: indented(t, plan), RatePlanVersion: 3}},
			tenantB: {"rmB": {ID: "rmB", Code: "201", BuildingID: "bB", StoredStatus: string(room.StatusVacant), RatePlan: plan.Snapshot()}},
		}},
		levels: &fakeLevels{levels: map[string]access.Level{bldgA: access.EDIT}},
		idem:   &fakeIdem{m: map[string]idemState{}},
		audit:  &fakeAudit{},
		caller: Caller{TenantID: tenantA, UserID: "u1", Role: access.RoleReceptionist},
	}
	e.uow = &rollbackUoW{idem: e.idem}
	e.s = NewStays(e.uow, e.repo, e.levels, e.enc, e.idem, e.audit, &seqIDs{}, fixedClock{t0})
	return e
}

func indented(t *testing.T, p pricing.RatePlan) []byte {
	t.Helper()
	b := []byte(strings.ReplaceAll(string(p.Snapshot()), ",", ",\n  "))
	if _, err := pricing.ParseRatePlan(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func (e *stayEnv) setStatus(s room.Status) {
	rm := e.repo.rooms[tenantA][roomA]
	rm.StoredStatus = string(s)
	e.repo.rooms[tenantA][roomA] = rm
}

func goodInput() CreateStayInput {
	id := idMarker
	return CreateStayInput{RentalType: "HOURLY", GuestName: "Marker Guest", GuestPhone: "+84 900 000 111", IDNumber: &id, Deposit: 200_000}
}

func mustLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(zoneName)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func (r *fakeStayRepo) PendingPayment(context.Context, Tx, string) (*PendingPayment, error) {
	return nil, nil
}

func (r *fakeStayRepo) InvoiceByStay(context.Context, Tx, string) (InvoiceRecord, bool, error) {
	if r.invoice == nil {
		return InvoiceRecord{}, false, nil
	}
	return *r.invoice, true, nil
}
