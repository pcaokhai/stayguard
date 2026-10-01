package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

func (e *billEnv) checkout(id, key string) (InvoiceView, bool, error) {
	return e.b.Checkout(context.Background(), e.caller, id, key)
}

var checkoutExtras = []ExtraRecord{{ServiceCode: "WATER", Name: LocalizedName{VI: "Nuoc", EN: "Water"}, Quantity: 2, UnitAmount: 15_000}}

func TestCheckout_SG205_AC3(t *testing.T) {
	e := newBillEnv(t)
	rec := e.seedBillStay(t, StayRecord{Deposit: 100_000, Extras: checkoutExtras})
	e.clock.now = rec.CheckInAt.Add(3*time.Hour + 20*time.Minute)
	v, replayed, err := e.checkout("st1", callID(1))
	if err != nil || replayed {
		t.Fatalf("err=%v replayed=%v", err, replayed)
	}
	got := e.repo.records[tenantA]["st1"]
	if got.Status != "CHECKED_OUT" || got.CheckOutAt == nil || !got.CheckOutAt.Equal(e.clock.now) {
		t.Fatalf("stay: %+v", got)
	}
	if v.BillCode != "PH0930A101" || v.Status != "OPEN" || v.StayID != "st1" || v.RoomCode != "A101" || v.ID == "" || !v.CreatedAt.Equal(e.clock.now) {
		t.Fatalf("invoice: %+v", v)
	}
	if want := expectedDetail(t, rec, e.clock.now); !reflect.DeepEqual(v.Quote, want) {
		t.Fatalf("frozen quote differs from QuoteRunning + Assemble:\n%+v\n%+v", v.Quote, want)
	}
	stored := e.repo.invoices[tenantA]["st1"]
	var frozen QuoteView
	if err := json.Unmarshal(stored.Quote, &frozen); err != nil || !reflect.DeepEqual(frozen, v.Quote) || stored.Total != v.Quote.Total || stored.ID != v.ID {
		t.Fatalf("stored invoice: %v %+v", err, stored)
	}
	if len(e.audit.entries) != 1 {
		t.Fatalf("audit entries: %d", len(e.audit.entries))
	}
	a := e.audit.entries[0]
	if a.Action != "stay.checked_out" || a.EntityType != "stay" || a.EntityID != "st1" || a.ActorID != "u1" ||
		!strings.Contains(string(a.After), `"billCode":"PH0930A101"`) || !strings.Contains(string(a.After), v.ID) {
		t.Fatalf("audit: %+v %s", a, a.After)
	}
}

func TestCheckoutRepeat_SG205_AC3(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{Extras: checkoutExtras})
	e.clock.now = e.repo.records[tenantA]["st1"].CheckInAt.Add(2 * time.Hour)
	retryID := callID(1)
	first, _, err := e.checkout("st1", retryID)
	if err != nil {
		t.Fatal(err)
	}
	again, replayed, err := e.checkout("st1", retryID)
	if err != nil || !replayed || !reflect.DeepEqual(first, again) {
		t.Fatalf("same key: %v %v", err, replayed)
	}
	e.clock.now = e.clock.now.Add(72 * time.Hour)
	other, replayed, err := e.checkout("st1", callID(2))
	if err != nil || replayed || !reflect.DeepEqual(first, other) {
		t.Fatalf("different key must return the same invoice: %v %v\n%+v\n%+v", err, replayed, first, other)
	}
	if e.repo.insertedInvs != 1 || len(e.repo.invoices[tenantA]) != 1 || e.repo.markCount != 1 || len(e.audit.entries) != 1 {
		t.Fatalf("repeat wrote again: invoices=%d marks=%d audits=%d", e.repo.insertedInvs, e.repo.markCount, len(e.audit.entries))
	}
	if _, _, err := e.checkout("st1", callID(2)); err != nil {
		t.Fatalf("the second key must now replay: %v", err)
	}
	for _, k := range []string{"", strings.Repeat("k", 65)} {
		if _, _, err := e.checkout("st1", k); !errors.Is(err, ErrInvalidIdempotencyKey) {
			t.Fatalf("key of %d bytes: %v", len(k), err)
		}
	}
}

func TestCheckoutMissingInvoice_SG205_AC3(t *testing.T) {
	e := newBillEnv(t)
	out := e.clock.now
	e.seedBillStay(t, StayRecord{Status: "CHECKED_OUT", CheckOutAt: &out})
	if _, _, err := e.checkout("st1", callID(1)); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("a checked-out stay without an invoice is a server error: %v", err)
	}
}

func TestCheckoutBillCodeCollision_SG205_AC3(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{ID: "st1"})
	e.seedBillStay(t, StayRecord{ID: "st2"})
	e.clock.now = e.repo.records[tenantA]["st1"].CheckInAt.Add(time.Hour)
	a, _, err := e.checkout("st1", callID(1))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := e.checkout("st2", callID(2))
	if err != nil || a.BillCode != "PH0930A101" || b.BillCode != "PH0930A1012" {
		t.Fatalf("codes %q %q %v", a.BillCode, b.BillCode, err)
	}
}

func TestCheckoutBillCodeUsesTenantZone_SG205_AC3(t *testing.T) {
	e := newBillEnv(t)
	rec := e.seedBillStay(t, StayRecord{})
	e.clock.now = time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC) // 01:00 on 1 October in Ho Chi Minh City
	if !e.clock.now.After(rec.CheckInAt) {
		t.Fatal("test setup")
	}
	v, _, err := e.checkout("st1", callID(1))
	if err != nil || v.BillCode != "PH1001A101" {
		t.Fatalf("%q %v", v.BillCode, err)
	}
}

func TestInvoiceQuoteFrozen_SG205_AC5(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{Deposit: 500_000})
	e.clock.now = e.repo.records[tenantA]["st1"].CheckInAt.Add(5 * time.Hour)
	first, _, err := e.checkout("st1", callID(1))
	if err != nil {
		t.Fatal(err)
	}
	before := string(e.repo.invoices[tenantA]["st1"].Quote)
	e.clock.now = e.clock.now.Add(30 * 24 * time.Hour)
	later, _, err := e.checkout("st1", callID(2))
	if err != nil || !reflect.DeepEqual(first.Quote, later.Quote) || string(e.repo.invoices[tenantA]["st1"].Quote) != before {
		t.Fatalf("quote moved with the clock: %v", err)
	}
	if first.Quote.RefundDue == 0 {
		t.Fatalf("the deposit exceeds the total, a refund is due: %+v", first.Quote)
	}
}

func TestCheckoutRoomStaysOccupied_SG205_AC6(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{})
	e.clock.now = e.repo.records[tenantA]["st1"].CheckInAt.Add(time.Hour)
	if _, _, err := e.checkout("st1", callID(1)); err != nil {
		t.Fatal(err)
	}
	if got := e.repo.rooms[tenantA][roomA].StoredStatus; got != string(room.StatusOccupied) {
		t.Fatalf("room status = %s", got)
	}
	_, _, err := e.se.s.CreateStay(context.Background(), e.caller, roomA, callID(3), goodInput())
	if !errors.Is(err, room.ErrNotVacant) {
		t.Fatalf("check-in on the unpaid room: %v", err)
	}
}

func TestInvoiceViewJSON_SG205_AC3(t *testing.T) {
	v := InvoiceView{ID: "iv1", StayID: "st1", RoomCode: "A101", BillCode: "PH0930A101", Status: "OPEN",
		CreatedAt: time.Date(2026, 9, 30, 6, 20, 0, 123, time.UTC),
		Quote: QuoteView{AsOf: time.Date(2026, 9, 30, 6, 20, 0, 123, time.UTC), StayAmount: 1, ExtrasAmount: 2, Total: 3, DepositPaid: 4,
			BalanceDue: 5, RefundDue: 6, Capped: true, Lines: []LineView{{"DAILY", 1, 350_000, 350_000}}}}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back InvoiceView
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(v, back) {
		t.Fatalf("round trip: %v\n%+v\n%+v", err, v, back)
	}
}
