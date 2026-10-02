package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	svcWater = "sv_water"
	svcBeer  = "sv_beer"
)

// fakeServiceRepo scopes by the tenant of the tx and guards stock like the adapter's UPDATE.
type fakeServiceRepo struct {
	rows     map[string]map[string]ServiceRow
	calls    int
	decOrder []string
	sales    []string
	// livePrice replaces the price at decrement time, after ByCodes has read the old one.
	livePrice map[string]int64
}

func (r *fakeServiceRepo) List(_ context.Context, tx Tx, _ bool, _ time.Time) ([]Service, error) {
	r.calls++
	var out []Service
	for _, row := range r.rows[tx.TenantID()] {
		out = append(out, row.Service)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (r *fakeServiceRepo) ByCodes(_ context.Context, tx Tx, codes []string) ([]ServiceRow, error) {
	r.calls++
	var out []ServiceRow
	for _, row := range r.rows[tx.TenantID()] {
		for _, c := range codes {
			if row.Code == c {
				out = append(out, row)
			}
		}
	}
	return out, nil
}

// DecrementStock returns the price the row has now; livePrice simulates a price change after ByCodes read it.
func (r *fakeServiceRepo) DecrementStock(_ context.Context, tx Tx, id string, qty int64) (int64, error) {
	r.calls++
	row, ok := r.rows[tx.TenantID()][id]
	if !ok || row.Stock < qty {
		return 0, stay.ErrInsufficientStock
	}
	row.Stock -= qty
	if p, ok := r.livePrice[id]; ok {
		row.Price = p
	}
	r.rows[tx.TenantID()][id] = row
	r.decOrder = append(r.decOrder, row.Code)
	return row.Price, nil
}

func (r *fakeServiceRepo) RecordSale(_ context.Context, _ Tx, _, serviceID string, qty int64, stayID, _ string, _ time.Time) error {
	r.sales = append(r.sales, fmt.Sprintf("%s:%d:%s", serviceID, qty, stayID))
	return nil
}

func (r *fakeServiceRepo) stock(tenant, id string) int64 { return r.rows[tenant][id].Stock }

// fakeBillingRepo adds the billing writes to the stay fake, with the real semantics: one invoice
// per stay, bill codes unique per tenant, extras only on ACTIVE stays.
type fakeBillingRepo struct {
	*fakeStayRepo
	svc          *fakeServiceRepo
	invoices     map[string]map[string]InvoiceRecord // tenant, then stay id
	markCount    int
	insertedInvs int
	probes       int
	extraRows    []NewExtra
	insertTries  int
	conflicts    int // the next InsertInvoice calls fail with ErrBillCodeConflict
}

func (r *fakeBillingRepo) LockStay(ctx context.Context, tx Tx, id string) (StayRecord, bool, error) {
	r.locks++
	return r.StayByID(ctx, tx, id)
}

func (r *fakeBillingRepo) InsertExtra(_ context.Context, tx Tx, e NewExtra) error {
	r.calls++
	r.extraRows = append(r.extraRows, e)
	rec := r.records[tx.TenantID()][e.StayID]
	svc := r.svc.rows[tx.TenantID()][e.ServiceID]
	rec.Extras = append(append([]ExtraRecord(nil), rec.Extras...), ExtraRecord{ServiceCode: svc.Code, Name: svc.Name, Quantity: e.Quantity, UnitAmount: e.UnitAmount})
	r.records[tx.TenantID()][e.StayID] = rec
	return nil
}

func (r *fakeBillingRepo) MarkCheckedOut(_ context.Context, tx Tx, id string, at time.Time) error {
	r.calls++
	rec := r.records[tx.TenantID()][id]
	if rec.Status != string(stay.StatusActive) {
		return stay.ErrNotActive
	}
	rec.Status, rec.CheckOutAt = string(stay.StatusCheckedOut), &at
	r.records[tx.TenantID()][id] = rec
	r.markCount++
	return nil
}

func (r *fakeBillingRepo) InvoiceByStay(_ context.Context, tx Tx, id string) (InvoiceRecord, bool, error) {
	r.calls++
	inv, ok := r.invoices[tx.TenantID()][id]
	return inv, ok, nil
}

func (r *fakeBillingRepo) InsertInvoice(_ context.Context, tx Tx, n NewInvoice) error {
	r.calls++
	r.insertTries++
	if r.conflicts > 0 {
		r.conflicts--
		return ErrBillCodeConflict
	}
	mine := r.invoices[tx.TenantID()]
	for _, inv := range mine {
		if inv.StayID == n.StayID || inv.BillCode == n.BillCode {
			return fmt.Errorf("duplicate invoice")
		}
	}
	mine[n.StayID] = InvoiceRecord{ID: n.ID, StayID: n.StayID, BillCode: n.BillCode, Status: "OPEN", Quote: n.Quote, Total: n.Total, CreatedAt: n.CreatedAt}
	r.insertedInvs++
	return nil
}

func (r *fakeBillingRepo) BillCodeTaken(_ context.Context, tx Tx, code string) (bool, error) {
	r.calls++
	r.probes++
	for _, inv := range r.invoices[tx.TenantID()] {
		if inv.BillCode == code {
			return true, nil
		}
	}
	return false, nil
}

// billingUoW discards every fake write of a failed unit of work, like a rolled back transaction.
type billingUoW struct {
	fakeUoW
	e *billEnv
}

type billSnapshot struct {
	idem     map[string]idemState
	records  map[string]StayRecord
	invoices map[string]InvoiceRecord
	stock    map[string]ServiceRow
	audits   int
}

func (u *billingUoW) Do(ctx context.Context, tenantID string, fn func(context.Context, Tx) error) error {
	snap := u.e.snapshot()
	err := u.fakeUoW.Do(ctx, tenantID, fn)
	if err != nil {
		u.e.restore(snap)
	}
	return err
}

func (e *billEnv) snapshot() billSnapshot {
	s := billSnapshot{idem: e.idem.snapshot(), records: map[string]StayRecord{}, invoices: map[string]InvoiceRecord{},
		stock: map[string]ServiceRow{}, audits: len(e.audit.entries)}
	for k, v := range e.repo.records[tenantA] {
		s.records[k] = v
	}
	for k, v := range e.repo.invoices[tenantA] {
		s.invoices[k] = v
	}
	for k, v := range e.svc.rows[tenantA] {
		s.stock[k] = v
	}
	return s
}

func (e *billEnv) restore(s billSnapshot) {
	e.idem.m = s.idem
	e.repo.records[tenantA], e.repo.invoices[tenantA], e.svc.rows[tenantA] = s.records, s.invoices, s.stock
	e.audit.entries = e.audit.entries[:s.audits]
}

// movableClock lets a test move time after a check-out.
type movableClock struct{ now time.Time }

func (c *movableClock) Now() time.Time { return c.now }

type billEnv struct {
	b      *Billing
	se     *stayEnv // the check-in side sharing the same repo, for the room tests
	repo   *fakeBillingRepo
	svc    *fakeServiceRepo
	levels *fakeLevels
	idem   *fakeIdem
	audit  *fakeAudit
	clock  *movableClock
	caller Caller
}

func newBillEnv(t *testing.T) *billEnv {
	t.Helper()
	se := newStayEnv(t)
	e := &billEnv{se: se, levels: se.levels, idem: se.idem, audit: se.audit, caller: se.caller, clock: &movableClock{t0}}
	e.svc = &fakeServiceRepo{rows: map[string]map[string]ServiceRow{
		tenantA: {
			svcWater: {ID: svcWater, Service: Service{Code: "WATER", Name: LocalizedName{VI: "Nuoc", EN: "Water"}, Price: 15_000, Stock: 5}},
			svcBeer:  {ID: svcBeer, Service: Service{Code: "BEER", Name: LocalizedName{VI: "Bia", EN: "Beer"}, Price: 25_000, Stock: 3}},
		},
		tenantB: {"sv_bw": {ID: "sv_bw", Service: Service{Code: "WATER", Name: LocalizedName{EN: "Other"}, Price: 1, Stock: 1}}},
	}}
	e.repo = &fakeBillingRepo{fakeStayRepo: se.repo, svc: e.svc,
		invoices: map[string]map[string]InvoiceRecord{tenantA: {}, tenantB: {}}}
	e.b = NewBilling(&billingUoW{e: e}, e.repo, e.svc, e.levels, fakeEnc{}, e.idem, e.audit, &seqIDs{}, e.clock)
	return e
}

// seedBillStay stores an ACTIVE hourly stay in room A101 unless rec says otherwise.
func (e *billEnv) seedBillStay(t *testing.T, rec StayRecord) StayRecord {
	t.Helper()
	if rec.ID == "" {
		rec.ID = "st1"
	}
	if rec.RentalType == "" {
		rec.RentalType = "HOURLY"
	}
	if rec.Status == "" {
		rec.Status = string(stay.StatusActive)
	}
	if rec.CheckInAt.IsZero() {
		rec.CheckInAt = time.Date(2026, 9, 30, 10, 0, 0, 0, mustLoc(t))
	}
	rec.GuestName, rec.GuestPhone, rec.BuildingID, rec.RoomID, rec.RoomCode = "G", "123456", bldgA, roomA, "A101"
	if rec.RatePlanSnapshot == nil {
		rec.RatePlanSnapshot = stayPlan().Snapshot()
	}
	e.repo.records[tenantA][rec.ID] = rec
	e.se.setStatus(room.StatusOccupied)
	return rec
}

func (e *billEnv) setLevel(l access.Level) { e.levels.levels[bldgA] = l }

// callID builds an idempotency key at run time: gitleaks flags uuid literals assigned to a key.
func callID(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", 0xabc00000+n, n) }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
