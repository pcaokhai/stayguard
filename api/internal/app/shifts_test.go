package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/shift"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

type fakeShiftRepo struct {
	rostered  []string
	shifts    []*ShiftRecord
	entries   map[string][]CashEntry
	lastFloat int64
	unpaid    []InvoiceCandidate
}

func (r *fakeShiftRepo) Timezone(context.Context, Tx) (string, error) { return zoneName, nil }
func (r *fakeShiftRepo) BuildingIDs(context.Context, Tx) ([]string, error) {
	return []string{bldgA}, nil
}
func (r *fakeShiftRepo) Scheduled(context.Context, Tx, string, time.Time) ([]string, error) {
	return r.rostered, nil
}
func (r *fakeShiftRepo) LockOpen(_ context.Context, _ Tx, userID string) (ShiftRecord, bool, error) {
	for _, s := range r.shifts {
		if s.UserID == userID && s.Status == shiftOpen {
			return *s, true, nil
		}
	}
	return ShiftRecord{}, false, nil
}
func (r *fakeShiftRepo) LockOpenForStay(context.Context, Tx, string) (ShiftRecord, bool, error) {
	for _, s := range r.shifts {
		if s.Status == shiftOpen {
			return *s, true, nil
		}
	}
	return ShiftRecord{}, false, nil
}
func (r *fakeShiftRepo) Open(_ context.Context, _ Tx, n NewShift) (bool, error) {
	r.shifts = append(r.shifts, &ShiftRecord{ID: n.ID, UserID: n.UserID, UserName: "Lan", Status: shiftOpen, Code: n.Code, OpenedAt: n.OpenedAt, OpeningFloat: n.OpeningFloat})
	return true, nil
}
func (r *fakeShiftRepo) LastFloatLeft(context.Context, Tx) (int64, error) { return r.lastFloat, nil }
func (r *fakeShiftRepo) AddEntry(_ context.Context, _ Tx, e CashEntry) error {
	for _, s := range r.shifts {
		if s.ID == e.ShiftID && s.Status != shiftOpen {
			return errors.New("cash entries need an open shift") // the trigger of the real table
		}
	}
	r.entries[e.ShiftID] = append(r.entries[e.ShiftID], e)
	return nil
}
func (r *fakeShiftRepo) Cash(_ context.Context, _ Tx, id string) (int64, int64, error) {
	var in, out int64
	for _, e := range r.entries[id] {
		if e.Kind == shift.Deposit || e.Kind == shift.Payment {
			in += e.Amount
		} else {
			out += e.Amount
		}
	}
	return in, out, nil
}
func (r *fakeShiftRepo) Transfers(context.Context, Tx, time.Time, time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeShiftRepo) UnpaidInvoices(context.Context, Tx, time.Time, time.Time) ([]InvoiceCandidate, error) {
	return r.unpaid, nil
}
func (r *fakeShiftRepo) Close(_ context.Context, _ Tx, c ShiftClose) error {
	for _, s := range r.shifts {
		if s.ID == c.ShiftID && s.Status == shiftOpen {
			s.Status, s.ClosedAt = shiftClosed, &c.At
			s.Expected, s.Counted, s.Difference = &c.Expected, &c.Counted, &c.Diff
			if c.Reason != "" {
				s.Reason, s.ReasonAt = &c.Reason, &c.At
			}
			r.lastFloat = c.FloatLeft
			return nil
		}
	}
	return ErrShiftNotOpen
}
func (r *fakeShiftRepo) ByID(_ context.Context, _ Tx, id string) (ShiftRecord, bool, error) {
	for _, s := range r.shifts {
		if s.ID == id {
			return *s, true, nil
		}
	}
	return ShiftRecord{}, false, nil
}
func (r *fakeShiftRepo) Movements(context.Context, Tx, string) ([]CashMovement, error) {
	return nil, nil
}
func (r *fakeShiftRepo) CashIn(context.Context, Tx, string) ([]CashInRow, error) { return nil, nil }
func (r *fakeShiftRepo) MonthStats(context.Context, Tx, string, time.Time, time.Time) (int64, int64, error) {
	return 0, 0, nil
}
func (r *fakeShiftRepo) ListClosed(context.Context, Tx, ClosedFilter) ([]ClosedShiftRow, error) {
	return nil, nil
}

type shiftEnv struct {
	s      *Shifts
	repo   *fakeShiftRepo
	alerts *fakeAlerts
	lan    Caller
}

func newShiftEnv(openingFloat int64) *shiftEnv {
	repo := &fakeShiftRepo{entries: map[string][]CashEntry{}, lastFloat: openingFloat}
	e := &shiftEnv{repo: repo, alerts: &fakeAlerts{}, lan: Caller{TenantID: tenantA, UserID: "u_lan", Role: access.RoleReceptionist}}
	idem := &fakeIdem{m: map[string]idemState{}}
	e.s = NewShifts(&rollbackUoW{idem: idem}, repo, &fakeLevels{levels: map[string]access.Level{bldgA: access.EDIT}}, idem, &fakeAudit{}, e.alerts, &seqIDs{}, fixedClock{t0})
	return e
}

func (e *shiftEnv) record(t *testing.T, kind string, amount int64) {
	t.Helper()
	if err := e.s.Record(context.Background(), fakeTx{tenantA}, e.lan, CashRecord{Kind: kind, StayID: "st_1", Amount: amount}); err != nil {
		t.Fatal(err)
	}
}

// 100k float + 200k deposit + 300k balance - 50k refund - 30k payout = 520k.
func (e *shiftEnv) drawer(t *testing.T) {
	t.Helper()
	e.record(t, shift.Deposit, 200_000)
	e.record(t, shift.Payment, 300_000)
	e.record(t, shift.Refund, 50_000)
	if _, err := e.s.Payout(context.Background(), e.lan, "k-payout", PayoutInput{Amount: 30_000, Description: "ice"}); err != nil {
		t.Fatal(err)
	}
}

func counts520() []shift.Count {
	return []shift.Count{{Denomination: 500_000, Quantity: 1}, {Denomination: 20_000, Quantity: 1}}
}

func TestShift_ExpectedCashWithDepositsRefundsAndPayouts_SG503(t *testing.T) {
	e := newShiftEnv(100_000)
	if _, err := e.s.Current(context.Background(), e.lan); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before any cash action: %v", err)
	}
	e.drawer(t)
	v, err := e.s.Current(context.Background(), e.lan)
	if err != nil {
		t.Fatal(err)
	}
	if v.OpeningFloat != 100_000 || v.CashIn != 500_000 || v.CashOut != 80_000 || v.ExpectedCash != 520_000 || v.Status != "OPEN" || v.UserName != "Lan" {
		t.Fatalf("shift: %+v", v)
	}
	if len(e.repo.shifts) != 1 {
		t.Fatalf("one shift opened by the first cash action, got %d", len(e.repo.shifts))
	}
	if over := (PayoutInput{Amount: 520_001, Description: "too much"}); true {
		if _, err := e.s.Payout(context.Background(), e.lan, "k2", over); !isValidation(err, "amount") {
			t.Fatalf("payout above the drawer: %v", err)
		}
	}
}

func isValidation(err error, path string) bool {
	var ve *stay.ValidationError
	return errors.As(err, &ve) && ve.Errors[0].Path == path
}

func TestShift_OnlyReceptionistsHaveADrawer_SG503(t *testing.T) {
	e := newShiftEnv(0)
	for _, role := range []access.Role{access.RoleOwner, access.RoleManager} {
		c := e.lan
		c.Role = role
		if err := e.s.Record(context.Background(), fakeTx{tenantA}, c, CashRecord{Kind: shift.Payment, Amount: 1000}); err != nil || len(e.repo.shifts) != 0 {
			t.Fatalf("%s: %v shifts=%d", role, err, len(e.repo.shifts))
		}
		if _, err := e.s.Current(context.Background(), c); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s current: %v", role, err)
		}
	}
	hk := e.lan
	hk.Role = access.RoleHousekeeping
	if _, err := e.s.Current(context.Background(), hk); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("housekeeping: %v", err)
	}
}

func TestShift_CloseLocksFiguresAndNextShiftStartsWithTheFloat_SG503(t *testing.T) {
	e := newShiftEnv(100_000)
	e.drawer(t)
	ctx := context.Background()
	in := CloseShiftInput{Counts: counts520(), FloatLeft: 200_000}
	r, err := e.s.Close(ctx, e.lan, "k-close", in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Difference != 0 || r.CountedCash != 520_000 || r.Shift.Status != "CLOSED" || r.Shift.ExpectedCash != 520_000 || len(e.alerts.raised) != 0 {
		t.Fatalf("review: %+v alerts=%v", r, e.alerts.raised)
	}
	if again, err := e.s.Close(ctx, e.lan, "k-close", in); err != nil || again.Shift.ID != r.Shift.ID {
		t.Fatalf("replay: %v %+v", err, again)
	}
	if _, err := e.s.Close(ctx, e.lan, "k-close2", in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a second close finds no open shift: %v", err)
	}
	// The closed shift's ledger cannot take a line any more, and cash after the close opens a new shift with the float left.
	if err := e.repo.AddEntry(ctx, fakeTx{tenantA}, CashEntry{ID: "x", ShiftID: r.Shift.ID, Kind: shift.Payout, Amount: 1}); err == nil {
		t.Fatal("an entry was added to a closed shift")
	}
	e.record(t, shift.Payment, 10_000)
	v, err := e.s.Current(ctx, e.lan)
	if err != nil || v.ID == r.Shift.ID || v.OpeningFloat != 200_000 || v.ExpectedCash != 210_000 {
		t.Fatalf("next shift: %+v %v", v, err)
	}
	if n := len(e.repo.entries[r.Shift.ID]); n != 4 {
		t.Fatalf("closed shift ledger changed: %d entries", n)
	}
}

func TestShift_DifferenceNeedsReasonAndRaisesAlert_SG503(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		counts []shift.Count
		kind   string
		amount int64
	}{
		"short": {[]shift.Count{{Denomination: 500_000, Quantity: 1}, {Denomination: 10_000, Quantity: 1}}, AlertCashShort, 10_000},
		"over":  {[]shift.Count{{Denomination: 500_000, Quantity: 1}, {Denomination: 20_000, Quantity: 1}, {Denomination: 10_000, Quantity: 1}}, AlertCashOver, 10_000},
	} {
		e := newShiftEnv(100_000)
		e.drawer(t)
		if _, err := e.s.Close(ctx, e.lan, "k1", CloseShiftInput{Counts: c.counts, FloatLeft: 0}); !isValidation(err, "reason") {
			t.Fatalf("%s without a reason: %v", name, err)
		}
		if len(e.repo.shifts) != 1 || e.repo.shifts[0].Status != shiftOpen || len(e.alerts.raised) != 0 {
			t.Fatalf("%s: a refused close changed state", name)
		}
		r, err := e.s.Close(ctx, e.lan, "k2", CloseShiftInput{Counts: c.counts, Reason: "gave wrong change", FloatLeft: 0})
		if err != nil || r.Reason == nil || *r.Reason != "gave wrong change" || r.ReasonRecordedAt == nil {
			t.Fatalf("%s: %+v %v", name, r, err)
		}
		a := e.alerts.raised
		if len(a) != 1 || a[0].Kind != c.kind || *a[0].Amount != c.amount || a[0].ShiftID != r.Shift.ID || a[0].By != "u_lan" {
			t.Fatalf("%s alert: %+v", name, a)
		}
	}
}

func TestShift_CloseRefusals_SG503(t *testing.T) {
	e := newShiftEnv(100_000)
	e.drawer(t)
	ctx := context.Background()
	if _, err := e.s.Close(ctx, e.lan, "k1", CloseShiftInput{Counts: []shift.Count{{Denomination: 1234, Quantity: 1}}}); !isValidation(err, "counts") {
		t.Errorf("unknown denomination: %v", err)
	}
	if _, err := e.s.Close(ctx, e.lan, "k2", CloseShiftInput{Counts: counts520(), FloatLeft: 520_001}); !isValidation(err, "floatLeft") {
		t.Errorf("float above counted cash: %v", err)
	}
	if _, err := e.s.Payout(ctx, e.lan, "k3", PayoutInput{Amount: 0, Description: "x"}); !isValidation(err, "amount") {
		t.Errorf("zero payout: %v", err)
	}
	if _, err := e.s.Payout(ctx, e.lan, "k4", PayoutInput{Amount: 1000, Description: "  "}); !isValidation(err, "description") {
		t.Errorf("blank payout note: %v", err)
	}
}

func TestShift_ReviewIsForClosedShiftsOfTheOwner_SG503(t *testing.T) {
	e := newShiftEnv(0)
	e.record(t, shift.Payment, 20_000)
	ctx := context.Background()
	boss := Caller{TenantID: tenantA, UserID: "u_o", Role: access.RoleOwner}
	id := e.repo.shifts[0].ID
	if _, err := e.s.Review(ctx, boss, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an open shift has no review: %v", err)
	}
	if _, err := e.s.Close(ctx, e.lan, "k1", CloseShiftInput{Counts: []shift.Count{{Denomination: 20_000, Quantity: 1}}}); err != nil {
		t.Fatal(err)
	}
	if r, err := e.s.Review(ctx, boss, id); err != nil || r.CountedCash != 20_000 {
		t.Fatalf("review: %+v %v", r, err)
	}
	if _, err := e.s.Review(ctx, e.lan, id); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("receptionist: %v", err)
	}
}

// F-A3: the shift code of a new shift is the one the roster gives the person.
func TestShift_OpensWithTheRosteredShift_FA3(t *testing.T) {
	// t0 is 10:04 in Ho Chi Minh, which the hour calls MORNING.
	for name, c := range map[string]struct {
		rostered []string
		want     string
	}{
		"nobody rostered":       {nil, shift.Morning},
		"rostered for the hour": {[]string{shift.Morning, shift.Afternoon}, shift.Morning},
		"rostered for another":  {[]string{shift.Afternoon}, shift.Afternoon},
	} {
		e := newShiftEnv(0)
		e.repo.rostered = c.rostered
		e.record(t, shift.Payment, 1000)
		if got := e.repo.shifts[0].Code; got != c.want {
			t.Errorf("%s: %s, want %s", name, got, c.want)
		}
	}
}

// Anti-loss: a shift with invoices not fully paid lists them with what is left, and closing needs a reason even when the cash matches.
func TestShift_CloseWithUnpaidInvoicesNeedsReason_FU(t *testing.T) {
	e := newShiftEnv(100_000)
	e.drawer(t)
	e.repo.unpaid = []InvoiceCandidate{{InvoiceID: "iv_1", BillCode: "PH1002A101", RoomCode: "A101", Total: 180_000, Paid: 100_000, Balance: 80_000}}
	ctx := context.Background()
	v, err := e.s.Current(ctx, e.lan)
	if err != nil || len(v.UnpaidInvoices) != 1 || v.UnpaidInvoices[0].Balance != 80_000 {
		t.Fatalf("the shift lists the unpaid invoice with its remaining amount: %+v %v", v.UnpaidInvoices, err)
	}
	in := CloseShiftInput{Counts: counts520(), FloatLeft: 0}
	if _, err := e.s.Close(ctx, e.lan, "k-nr", in); !isValidation(err, "reason") {
		t.Fatalf("closing with unpaid invoices and no reason: %v", err)
	}
	in.Reason = "guest will transfer the rest tomorrow"
	r, err := e.s.Close(ctx, e.lan, "k-r", in)
	if err != nil || r.Shift.Status != "CLOSED" || len(r.Shift.UnpaidInvoices) != 1 {
		t.Fatalf("closing with a reason: %+v %v", r.Shift, err)
	}
	if len(e.alerts.raised) != 0 {
		t.Fatalf("matching cash raises no cash alert: %+v", e.alerts.raised)
	}
}
