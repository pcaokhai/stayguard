//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

const opsTenant = "tnt_ops"

// opsNow is 2026-10-02 10:00 in Ho Chi Minh; the seeded stay checked in at 08:30 there.
var opsNow = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)

type clockAt struct{ t time.Time }

func (c clockAt) Now() time.Time { return c.t }

// seqIDs is a deterministic id generator (an adapter may not import the ids adapter).
type seqIDs struct{ n int }

func (s *seqIDs) New(prefix string) string { s.n++; return fmt.Sprintf("%s_t%d", prefix, s.n) }

type editEverywhere struct{}

func (editEverywhere) Levels(_ context.Context, _ app.Caller, buildingIDs []string) (map[string]access.Level, error) {
	out := map[string]access.Level{}
	for _, id := range buildingIDs {
		out[id] = access.EDIT
	}
	return out, nil
}

func opsPlan(first int64) pricing.RatePlan {
	return pricing.RatePlan{
		Version: 1, Currency: pricing.CurrencyVND, GraceMinutes: 15,
		Hourly:    pricing.Hourly{FirstHour: money.Vnd(first), ExtraHour: 20_000},
		Overnight: pricing.Window{Price: 250_000, Start: pricing.Clock{Hour: 21}, End: pricing.Clock{Hour: 12}},
		Daily:     pricing.Window{Price: 350_000, Start: pricing.Clock{Hour: 14}, End: pricing.Clock{Hour: 12}},
	}
}

// seedOps adds real rate plans, one user and a HOURLY stay (A101, STD) checked in at 08:30 local time by that user.
func seedOps(t *testing.T, db, tenant string) {
	t.Helper()
	seedRoomTenant(t, db, tenant)
	owner := connAs(t, db, "owner")
	mustExec(t, owner, `UPDATE app.unit_types SET rate_plan = $1::jsonb WHERE id = $2`, string(opsPlan(80_000).Snapshot()), tenant+"_ut1")
	mustExec(t, owner, `UPDATE app.unit_types SET rate_plan = $1::jsonb WHERE id = $2`, string(opsPlan(150_000).Snapshot()), tenant+"_ut2")
	mustExec(t, owner, `INSERT INTO app.users (id, tenant_id, name, role) VALUES ($1, $2, 'Lan', 'RECEPTIONIST')`, tenant+"_lan", tenant)
	mustExec(t, owner, `UPDATE app.stays SET rental_type = 'HOURLY', check_in_at = $1, created_by = $2, rate_plan_snapshot = $3::jsonb, deposit = 50000 WHERE id = $4`,
		opsNow.Add(-90*time.Minute), tenant+"_lan", string(opsPlan(80_000).Snapshot()), tenant+"_s1")
}

func TestStayOps_EditMoveHistoryTimeline_SG801(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	seedOps(t, db, opsTenant)
	owner := connAs(t, db, "owner")
	uow := NewUnitOfWork(newAppPool(t, db, 4))
	clk := clockAt{opsNow}
	gen := &seqIDs{}
	idem, audit := NewIdempotencyStore(time.Hour), NewAuditWriter()
	lan := app.Caller{TenantID: opsTenant, UserID: opsTenant + "_lan", Role: access.RoleReceptionist}
	boss := app.Caller{TenantID: opsTenant, UserID: opsTenant + "_u", Role: access.RoleOwner}
	edits := app.NewStayEdits(uow, StayEditRepo{}, editEverywhere{}, nil, idem, audit, AlertWriter{}, gen, clk)
	history := app.NewStayHistory(uow, StayHistoryRepo{}, editEverywhere{}, clk)
	stayID := opsTenant + "_s1"

	// AC1 and AC3: an earlier check-in time prices more, and raises the alert.
	earlier := opsNow.Add(-4 * time.Hour)
	d, err := edits.EditCheckIn(ctx, lan, stayID, "k-edit", app.EditCheckInInput{NewCheckInAt: earlier, ReasonCode: "WRONG_TIME", Note: "typed the wrong time"})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	q, _ := pricing.QuoteRunning(opsPlan(80_000), pricing.RentalHourly, earlier, opsNow, mustZone(t))
	if !d.CheckInAt.Equal(earlier) || d.Quote.StayAmount != q.Total.Int64() {
		t.Fatalf("edited stay: checkIn %v amount %d want %d", d.CheckInAt, d.Quote.StayAmount, q.Total.Int64())
	}
	var kind, roomCode, oldTime string
	if err := owner.QueryRow(ctx, `SELECT kind, room_code, details->>'oldTime' FROM app.alerts WHERE tenant_id = $1 AND stay_id = $2`, opsTenant, stayID).
		Scan(&kind, &roomCode, &oldTime); err != nil || kind != "STAY_TIME_EDITED" || roomCode != "A101" || oldTime == "" {
		t.Fatalf("alert %q %q %q: %v", kind, roomCode, oldTime, err)
	}
	// a replay under the same key answers the same and raises nothing more
	if _, err := edits.EditCheckIn(ctx, lan, stayID, "k-edit", app.EditCheckInInput{NewCheckInAt: earlier, ReasonCode: "WRONG_TIME", Note: "typed the wrong time"}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	var alerts int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM app.alerts WHERE tenant_id = $1`, opsTenant).Scan(&alerts)
	if alerts != 1 {
		t.Fatalf("alerts after replay: %d", alerts)
	}

	// AC4: the move reprices with the VIP plan, frees A101 and takes A201 (a vacant VIP room).
	moved, err := edits.MoveStay(ctx, lan, stayID, "k-move", app.MoveStayInput{ToRoomID: opsTenant + "_u2", RentalType: "HOURLY"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	vip, _ := pricing.QuoteRunning(opsPlan(150_000), pricing.RentalHourly, earlier, opsNow, mustZone(t))
	if moved.RoomCode != "A201" || moved.Quote.StayAmount != vip.Total.Int64() || !moved.CheckInAt.Equal(earlier) {
		t.Fatalf("moved: %+v want amount %d", moved, vip.Total.Int64())
	}
	assertRoomStatus(ctx, t, owner, "A101", room.StatusToClean)
	assertRoomStatus(ctx, t, owner, "A201", room.StatusOccupied)
	if _, err := edits.MoveStay(ctx, lan, stayID, "k-move2", app.MoveStayInput{ToRoomID: opsTenant + "_u3", RentalType: "HOURLY"}); !errors.Is(err, room.ErrNotVacant) {
		t.Fatalf("move into a room under maintenance: %v", err)
	}

	// The history shows the stay as TIME_EDITED with the front desk name; a transfer left unpaid would show UNPAID.
	page, err := history.ListStays(ctx, boss, app.StayListQuery{Query: "a201"})
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "TIME_EDITED" || page.Items[0].FrontDeskName != "Lan" {
		t.Fatalf("history: %+v %v", page, err)
	}
	if page, _ = history.ListStays(ctx, boss, app.StayListQuery{Query: "zzz"}); len(page.Items) != 0 {
		t.Fatalf("search: %+v", page)
	}
	tl, err := history.Timeline(ctx, boss, stayID)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	var kinds []string
	for _, e := range tl {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) != 3 || kinds[0] != "CHECKED_IN" || kinds[1] != "CHECK_IN_EDITED" || kinds[2] != "MOVED" || tl[0].ActorName != "Lan" || tl[2].Details["to"] != "A201" {
		t.Fatalf("timeline kinds %v: %+v", kinds, tl)
	}
	// the original check-in time stays on the timeline even after the edit
	if want := opsNow.Add(-90 * time.Minute); !tl[0].At.Equal(want) {
		t.Fatalf("checked-in at %v want %v", tl[0].At, want)
	}
}

func mustZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func assertRoomStatus(ctx context.Context, t *testing.T, c *pgx.Conn, code string, want room.Status) {
	t.Helper()
	var got string
	if err := c.QueryRow(ctx, `SELECT status FROM app.units WHERE tenant_id = $1 AND code = $2`, opsTenant, code).Scan(&got); err != nil || got != string(want) {
		t.Fatalf("room %s is %q (%v), want %s", code, got, err, want)
	}
}

// A paid stay shows its method and a frozen receipt; a bank event that cannot settle raises owner alerts (SG-801 AC5).
func TestStayOps_ReceiptStatesAndBankAlerts_SG801(t *testing.T) {
	ctx := context.Background()
	const ten = "tnt_ops2"
	db := migratedDB(t)
	seedOps(t, db, "tnt_ops2")
	owner := connAs(t, db, "owner")
	uow := NewUnitOfWork(newAppPool(t, db, 4))
	clk := clockAt{opsNow}
	gen := &seqIDs{n: 1000} // the database is shared, so ids must not repeat the first test's
	boss := app.Caller{TenantID: ten, UserID: ten + "_u", Role: access.RoleOwner}
	history := app.NewStayHistory(uow, StayHistoryRepo{}, editEverywhere{}, clk)
	stayID, inv := ten+"_s1", ten+"_iv"

	quote := `{"asOf":"2026-10-02T03:00:00Z","stayAmount":100000,"extrasAmount":0,"total":100000,"depositPaid":50000,"balanceDue":50000,"refundDue":0,"capped":false,"lines":[{"code":"FIRST_HOUR","quantity":1,"unitAmount":80000,"amount":80000},{"code":"EXTRA_HOUR","quantity":1,"unitAmount":20000,"amount":20000}]}`
	mustExec(t, owner, `UPDATE app.stays SET status = 'CHECKED_OUT', check_out_at = $1 WHERE id = $2`, opsNow, stayID)
	mustExec(t, owner, `INSERT INTO app.invoices (id, tenant_id, stay_id, bill_code, quote, total, created_at) VALUES ($1, $2, $3, 'PH1002A101', $4::jsonb, 100000, $5)`, inv, ten, stayID, quote, opsNow)
	mustExec(t, owner, `INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount, reference_code) VALUES ($1, $2, $3, 'TRANSFER', 'PENDING', 50000, 'PH1002A101')`, ten+"_pm", ten, inv)

	if page, err := history.ListStays(ctx, boss, app.StayListQuery{}); err != nil || len(page.Items) != 1 || page.Items[0].State != "UNPAID" || page.Items[0].Total == nil || *page.Items[0].Total != 100_000 {
		t.Fatalf("unpaid history: %+v %v", page, err)
	}

	// The same bill code with part of the amount: PARTIAL event (state MISMATCH, no immediate alert). A transfer with no bill code: UNMATCHED_TRANSFER.
	pay := app.NewPayments(uow, PaymentRepo{}, editEverywhere{}, nil, NewIdempotencyStore(time.Hour), NewAuditWriter(), gen, clk).WithAlerts(AlertWriter{})
	if res, err := pay.Settle(ctx, app.PaymentEvent{TenantID: ten, Provider: "t", ExternalID: "e1", Content: "PH1002A101", Amount: 40_000, ReceivedAt: opsNow}); err != nil || res.Result != "PARTIAL" {
		t.Fatalf("partial: %+v %v", res, err)
	}
	if res, err := pay.Settle(ctx, app.PaymentEvent{TenantID: ten, Provider: "t", ExternalID: "e2", Content: "no code", Amount: 70_000, ReceivedAt: opsNow}); err != nil || res.Result != "UNMATCHED" {
		t.Fatalf("unmatched: %+v %v", res, err)
	}
	rows, err := owner.Query(ctx, `SELECT kind, coalesce(room_code, ''), amount FROM app.alerts WHERE tenant_id = $1 ORDER BY kind`, ten)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][2]any{}
	for rows.Next() {
		var k, rc string
		var amt int64
		if err := rows.Scan(&k, &rc, &amt); err != nil {
			t.Fatal(err)
		}
		got[k] = [2]any{rc, amt}
	}
	if got["UNMATCHED_TRANSFER"] != [2]any{"", int64(70_000)} || len(got) != 1 { // a short transfer raises nothing at once
		t.Fatalf("alerts: %v", got)
	}
	if page, _ := history.ListStays(ctx, boss, app.StayListQuery{State: "MISMATCH"}); len(page.Items) != 1 {
		t.Fatalf("mismatch filter: %+v", page)
	}
	tl, err := history.Timeline(ctx, boss, stayID)
	if err != nil || tl[len(tl)-1].Kind != "PAYMENT_MISMATCH" || tl[len(tl)-1].Details["received"] != "40000" {
		t.Fatalf("timeline: %+v %v", tl, err)
	}

	// Cash settles it: PAID with the method, and the receipt prints the frozen lines, not a re-pricing.
	mustExec(t, owner, `UPDATE app.payments SET status = 'EXPIRED' WHERE id = $1`, ten+"_pm")
	mustExec(t, owner, `INSERT INTO app.payments (id, tenant_id, invoice_id, method, status, amount, received_amount, paid_at) VALUES ($1, $2, $3, 'CASH', 'PAID', 50000, 50000, $4)`, ten+"_pc", ten, inv, opsNow)
	mustExec(t, owner, `UPDATE app.invoices SET status = 'PAID', paid_at = $1 WHERE id = $2`, opsNow, inv)
	page, err := history.ListStays(ctx, boss, app.StayListQuery{})
	if err != nil || page.Items[0].State != "PAID" || page.Items[0].PaymentMethod != "CASH" {
		t.Fatalf("paid history: %+v %v", page, err)
	}
	r, err := history.Receipt(ctx, boss, inv)
	if err != nil || r.BillCode != "PH1002A101" || r.Total != 100_000 || r.Deposit != 50_000 || len(r.Lines) != 2 || len(r.Payments) != 1 || r.Payments[0].Amount != 50_000 || r.PropertyName != "p" {
		t.Fatalf("receipt: %+v %v", r, err)
	}
	if _, err := history.Receipt(ctx, boss, "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown invoice: %v", err)
	}
	other := app.Caller{TenantID: "tnt_other", UserID: "x", Role: access.RoleOwner}
	seedRoomTenant(t, db, "tnt_other")
	if _, err := history.Receipt(ctx, other, inv); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("another tenant's invoice: %v", err)
	}
}
