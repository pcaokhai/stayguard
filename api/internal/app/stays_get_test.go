package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

func seedStay(e *stayEnv, rec StayRecord) {
	rec.GuestName, rec.GuestPhone, rec.BuildingID, rec.RoomID, rec.RoomCode = "G", "123456", bldgA, roomA, "101"
	rec.RatePlanSnapshot = stayPlan().Snapshot()
	e.repo.records[tenantA][rec.ID] = rec
}

func expectedDetail(t *testing.T, rec StayRecord, asOf time.Time) QuoteView {
	t.Helper()
	loc := mustLoc(t)
	rental, _ := pricing.ParseRentalType(rec.RentalType)
	end := asOf
	if !end.After(rec.CheckInAt) {
		end = rec.CheckInAt.Add(time.Minute)
	}
	q, err := pricing.Price(stayPlan(), rental, rec.CheckInAt, end, loc)
	if err != nil {
		t.Fatal(err)
	}
	var extras []pricing.Extra
	for _, x := range rec.Extras {
		extras = append(extras, pricing.Extra{ServiceCode: x.ServiceCode, Quantity: x.Quantity, UnitAmount: money.Vnd(x.UnitAmount)})
	}
	b, err := pricing.Assemble(q, extras, money.Vnd(rec.Deposit))
	if err != nil {
		t.Fatal(err)
	}
	var lines []LineView
	for _, l := range q.Lines {
		lines = append(lines, LineView{l.Code, l.Quantity, l.UnitAmount.Int64(), l.Amount.Int64()})
	}
	return QuoteView{AsOf: asOf.UTC(), StayAmount: b.StayTotal.Int64(), ExtrasAmount: b.ExtrasTotal.Int64(), Total: b.Total.Int64(),
		DepositPaid: rec.Deposit, BalanceDue: b.BalanceDue.Int64(), RefundDue: b.RefundDue.Int64(), Capped: q.Capped, Lines: lines}
}

func TestGetStayQuote_SG203_AC6(t *testing.T) {
	loc := mustLoc(t)
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, loc)
	out := in.Add(30 * time.Hour)
	extras := []ExtraRecord{{ServiceCode: "WATER", Name: LocalizedName{VI: "Nuoc", EN: "Water"}, Quantity: 2, UnitAmount: 15_000}}
	cases := map[string]struct {
		rec StayRecord
		now time.Time
	}{
		"hourly":          {StayRecord{RentalType: "HOURLY", Status: "ACTIVE", Deposit: 100_000}, in.Add(3*time.Hour + 20*time.Minute)},
		"overnight":       {StayRecord{RentalType: "OVERNIGHT", Status: "ACTIVE", Deposit: 500_000}, in.Add(14 * time.Hour)},
		"daily":           {StayRecord{RentalType: "DAILY", Status: "ACTIVE"}, in.Add(26 * time.Hour)},
		"just checked in": {StayRecord{RentalType: "HOURLY", Status: "ACTIVE", Deposit: 50_000}, in},
		"with extras":     {StayRecord{RentalType: "HOURLY", Status: "ACTIVE", Extras: extras}, in.Add(2 * time.Hour)},
		"checked out":     {StayRecord{RentalType: "OVERNIGHT", Status: "CHECKED_OUT", CheckOutAt: &out}, in.Add(90 * time.Hour)},
	}
	for name, tc := range cases {
		e := newStayEnv(t)
		e.s.clock = fixedClock{tc.now}
		tc.rec.ID, tc.rec.CheckInAt = "st1", in
		seedStay(e, tc.rec)
		got, err := e.s.GetStay(context.Background(), e.caller, "st1")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		asOf := tc.now
		if tc.rec.CheckOutAt != nil {
			asOf = *tc.rec.CheckOutAt
		}
		if want := expectedDetail(t, tc.rec, asOf); !reflect.DeepEqual(got.Quote, want) {
			t.Fatalf("%s:\n got %+v\nwant %+v", name, got.Quote, want)
		}
		if got.PricingVersion != 3 || got.Deposit != tc.rec.Deposit || len(got.Extras) != len(tc.rec.Extras) {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	e := newStayEnv(t)
	e.s.clock = fixedClock{in}
	seedStay(e, StayRecord{ID: "st1", RentalType: "HOURLY", Status: "ACTIVE", CheckInAt: in, Extras: extras})
	got, _ := e.s.GetStay(context.Background(), e.caller, "st1")
	if got.Quote.ExtrasAmount != 30_000 || got.Extras[0].Amount != 30_000 || got.Quote.Total != got.Quote.StayAmount+30_000 {
		t.Fatalf("extras: %+v", got.Quote)
	}
	if got.Quote.StayAmount != 80_000 { // just checked in: the minimum billable amount
		t.Fatalf("minimum billable: %d", got.Quote.StayAmount)
	}
}

func TestGetStayFailsClosed_SG203_AC6(t *testing.T) {
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, mustLoc(t))
	for name, rec := range map[string]StayRecord{
		"unknown status":      {Status: "WEIRD"},
		"checked out no time": {Status: "CHECKED_OUT"},
		"unknown rental":      {Status: "ACTIVE", RentalType: "WEEKLY"},
	} {
		e := newStayEnv(t)
		rec.ID, rec.CheckInAt = "st1", in
		if rec.RentalType == "" {
			rec.RentalType = "HOURLY"
		}
		seedStay(e, rec)
		_, err := e.s.GetStay(context.Background(), e.caller, "st1")
		if err == nil || strings.Contains(err.Error(), "WEEKLY") {
			t.Fatalf("%s: expected an error without the stored value: %v", name, err)
		}
		if name == "unknown rental" && !errors.Is(err, pricing.ErrUnknownRentalType) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestGetStayAccess_SG203_AC6(t *testing.T) {
	in := time.Date(2026, 10, 5, 10, 0, 0, 0, mustLoc(t))
	e := newStayEnv(t)
	seedStay(e, StayRecord{ID: "st1", RentalType: "HOURLY", Status: "ACTIVE", CheckInAt: in})
	e.repo.records[tenantB]["stB"] = StayRecord{ID: "stB", BuildingID: "bB"}
	ctx := context.Background()

	h := e.caller
	h.Role = access.RoleHousekeeping
	if _, err := e.s.GetStay(ctx, h, "st1"); !errors.Is(err, access.ErrRoleForbidden) || e.repo.calls != 0 {
		t.Fatalf("role: %v", err)
	}
	e.levels.levels[bldgA] = access.NONE
	if _, err := e.s.GetStay(ctx, e.caller, "st1"); !errors.Is(err, access.ErrBuildingForbidden) {
		t.Fatalf("NONE: %v", err)
	}
	e.levels.levels[bldgA] = access.VIEW // VIEW is enough to read
	if _, err := e.s.GetStay(ctx, e.caller, "st1"); err != nil {
		t.Fatalf("VIEW: %v", err)
	}
	_, foreign := e.s.GetStay(ctx, e.caller, "stB")
	_, unknown := e.s.GetStay(ctx, e.caller, "nope")
	if !errors.Is(foreign, ErrNotFound) || foreign.Error() != unknown.Error() {
		t.Fatalf("foreign=%v unknown=%v", foreign, unknown)
	}
}

func TestStayDeposit_SG203_AC5(t *testing.T) {
	e := newStayEnv(t)
	v, _, err := create(e, "k1", goodInput())
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.s.GetStay(context.Background(), e.caller, v.ID)
	if err != nil || v.Deposit != 200_000 || got.Deposit != 200_000 || v.Quote.DepositPaid != 200_000 {
		t.Fatalf("deposit: %v %+v %+v", err, v, got)
	}
	if v.Quote.RefundDue != 200_000-v.Quote.Total || e.repo.inserted[0].Deposit != 200_000 || len(e.audit.entries) != 1 {
		t.Fatalf("settlement or side effect: %+v", v.Quote)
	}
	in := goodInput()
	in.Deposit = stay.MaxDeposit + 1
	if _, _, err := create(newStayEnv(t), "k", in); err == nil {
		t.Fatal("deposit above the maximum must be rejected")
	}
}

func TestStayViewJSON_SG203_AC3(t *testing.T) {
	out := time.Date(2026, 10, 6, 1, 2, 3, 456, time.UTC)
	m := "*****345"
	v := StayDetail{ID: "st1", RoomID: "r", RoomCode: "101", RentalType: "DAILY", Status: "CHECKED_OUT",
		CheckInAt: time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC), CheckOutAt: &out, GuestName: "Nguyễn Văn A", GuestPhone: "+84 1",
		IDNumberMasked: &m, Deposit: 9, PricingVersion: 2,
		Extras: []ExtraView{{"W", LocalizedName{"Nước", "Water"}, 2, 15_000, 30_000}},
		Quote: QuoteView{AsOf: out, StayAmount: 1, ExtrasAmount: 2, Total: 3, DepositPaid: 4, BalanceDue: 5, RefundDue: 6, Capped: true,
			Lines: []LineView{{"DAILY", 1, 350_000, 350_000}}}}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back StayDetail
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(v, back) {
		t.Fatalf("round trip: %v\n%+v\n%+v", err, v, back)
	}
	var none StayDetail
	raw, _ = json.Marshal(none)
	var backNone StayDetail
	if err := json.Unmarshal(raw, &backNone); err != nil || backNone.IDNumberMasked != nil || backNone.CheckOutAt != nil {
		t.Fatalf("nil pointers must stay nil: %v", err)
	}
}
