package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

type fakeHistoryRepo struct {
	buildings []string
	rows      []StayListRow
	got       StayFilter
	receipt   ReceiptRecord
}

func (r *fakeHistoryRepo) Timezone(context.Context, Tx) (string, error)      { return zoneName, nil }
func (r *fakeHistoryRepo) BuildingIDs(context.Context, Tx) ([]string, error) { return r.buildings, nil }
func (r *fakeHistoryRepo) Stays(_ context.Context, _ Tx, f StayFilter) ([]StayListRow, error) {
	r.got = f
	return r.rows, nil
}
func (r *fakeHistoryRepo) StayBuilding(_ context.Context, _ Tx, id string) (string, bool, error) {
	return bldgA, id == editStay, nil
}
func (r *fakeHistoryRepo) Timeline(context.Context, Tx, string) ([]TimelineRow, error) {
	return []TimelineRow{{Kind: "CHECKED_IN"}}, nil
}
func (r *fakeHistoryRepo) Receipt(_ context.Context, _ Tx, id string) (ReceiptRecord, bool, error) {
	return r.receipt, id == "iv_1", nil
}

func newHistoryEnv(role access.Role) (*StayHistory, *fakeHistoryRepo, Caller) {
	repo := &fakeHistoryRepo{buildings: []string{bldgA, bldgB}}
	levels := &fakeLevels{levels: map[string]access.Level{bldgA: access.VIEW, bldgB: access.NONE}}
	return NewStayHistory(&fakeUoW{}, repo, levels, fixedClock{t0}), repo, Caller{TenantID: tenantA, UserID: "u1", Role: role}
}

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// t0 is 2026-01-02 10:04 in Ho Chi Minh, so today is 2 January and the seven-day window starts on 26 December.
func TestListStays_ReceptionistWindow_SG802(t *testing.T) {
	h, repo, c := newHistoryEnv(access.RoleReceptionist)
	ctx := context.Background()
	for name, q := range map[string]StayListQuery{
		"eight days ago":  {Date: day(2025, 12, 25)},
		"range too early": {From: day(2025, 12, 20), To: day(2026, 1, 2)},
	} {
		_, err := h.ListStays(ctx, c, q)
		var ve *stay.ValidationError
		if !errors.As(err, &ve) || ve.Errors[0].Code != stay.CodeMin {
			t.Errorf("%s: got %v, want a 422 MIN", name, err)
		}
	}
	for name, q := range map[string]StayListQuery{"today": {}, "seven days ago": {Date: day(2025, 12, 26)}, "yesterday": {Date: day(2026, 1, 1)}} {
		if _, err := h.ListStays(ctx, c, q); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// today in Ho Chi Minh: [2026-01-02 00:00 +07, 2026-01-03 00:00 +07)
	if _, err := h.ListStays(ctx, c, StayListQuery{}); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC); !repo.got.From.Equal(want) || !repo.got.To.Equal(want.Add(24*time.Hour)) {
		t.Fatalf("window %v - %v", repo.got.From, repo.got.To)
	}
}

func TestListStays_OwnerAndManagerHaveNoWindow_SG802(t *testing.T) {
	for _, role := range []access.Role{access.RoleOwner, access.RoleManager} {
		h, _, c := newHistoryEnv(role)
		if _, err := h.ListStays(context.Background(), c, StayListQuery{From: day(2025, 6, 1), To: day(2025, 6, 30)}); err != nil {
			t.Errorf("%s: %v", role, err)
		}
	}
}

func TestListStays_BuildingsAndGuards_SG802(t *testing.T) {
	h, repo, c := newHistoryEnv(access.RoleReceptionist)
	ctx := context.Background()
	if _, err := h.ListStays(ctx, c, StayListQuery{}); err != nil || len(repo.got.BuildingIDs) != 1 || repo.got.BuildingIDs[0] != bldgA {
		t.Fatalf("only viewable buildings: %v %v", repo.got.BuildingIDs, err)
	}
	if _, err := h.ListStays(ctx, c, StayListQuery{BuildingID: bldgB}); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Errorf("building without access: %v", err)
	}
	hk := c
	hk.Role = access.RoleHousekeeping
	if _, err := h.ListStays(ctx, hk, StayListQuery{}); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("housekeeping: %v", err)
	}
	var ve *stay.ValidationError
	for name, q := range map[string]StayListQuery{"bad state": {State: "WEIRD"}, "bad cursor": {Cursor: "%%%"},
		"to before from": {From: day(2026, 1, 2), To: day(2026, 1, 1)}, "from without to": {From: day(2026, 1, 1)}} {
		if _, err := h.ListStays(ctx, c, q); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestListStays_PagingCursor_SG802(t *testing.T) {
	h, repo, c := newHistoryEnv(access.RoleOwner)
	for i := 0; i < historyPageSize+1; i++ {
		repo.rows = append(repo.rows, StayListRow{ID: fmt.Sprintf("st_%03d", i), CheckInAt: t0.Add(-time.Duration(i) * time.Minute)})
	}
	page, err := h.ListStays(context.Background(), c, StayListQuery{})
	if err != nil || len(page.Items) != historyPageSize || page.NextCursor == "" || repo.got.Limit != historyPageSize+1 {
		t.Fatalf("page: %d items cursor %q limit %d err %v", len(page.Items), page.NextCursor, repo.got.Limit, err)
	}
	if _, err := h.ListStays(context.Background(), c, StayListQuery{Cursor: page.NextCursor}); err != nil ||
		repo.got.CursorAt == nil || repo.got.CursorID != page.Items[historyPageSize-1].ID {
		t.Fatalf("cursor %+v %v", repo.got, err)
	}
	repo.rows = repo.rows[:3]
	if page, _ = h.ListStays(context.Background(), c, StayListQuery{}); page.NextCursor != "" || len(page.Items) != 3 {
		t.Fatalf("last page: %+v", page)
	}
}

func TestTimelineAndReceipt_Access_SG904(t *testing.T) {
	h, repo, c := newHistoryEnv(access.RoleReceptionist)
	ctx := context.Background()
	if _, err := h.Timeline(ctx, c, editStay); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("receptionist timeline: %v", err)
	}
	mgr := c
	mgr.Role = access.RoleManager
	if rows, err := h.Timeline(ctx, mgr, editStay); err != nil || len(rows) != 1 {
		t.Errorf("manager timeline: %v %v", rows, err)
	}
	if _, err := h.Timeline(ctx, mgr, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown stay: %v", err)
	}
	repo.receipt = ReceiptRecord{BuildingID: bldgA, BillCode: "PH0102101", RoomCode: "101", CheckInAt: t0, CheckOutAt: t0.Add(time.Hour),
		Quote:  []byte(`{"total":100000,"depositPaid":20000,"lines":[{"code":"FIRST_HOUR","quantity":1,"unitAmount":80000,"amount":80000}]}`),
		Extras: []ExtraRecord{{ServiceCode: "WATER", Quantity: 2, UnitAmount: 10_000}}, Payments: []PaidPayment{{ID: "pm1", Method: "CASH", Amount: 80_000, At: t0}}}
	v, err := h.Receipt(ctx, c, "iv_1")
	if err != nil || v.Total != 100_000 || v.Deposit != 20_000 || len(v.Lines) != 1 || v.Extras[0].Amount != 20_000 || v.Payments[0].Method != "CASH" {
		t.Fatalf("receipt: %+v %v", v, err)
	}
	if _, err := h.Receipt(ctx, c, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown invoice: %v", err)
	}
	h.levels = &fakeLevels{levels: map[string]access.Level{bldgA: access.NONE}}
	if _, err := h.Receipt(ctx, c, "iv_1"); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Errorf("no building access: %v", err)
	}
}
