package app

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func keysOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The frozen invoice quote and the stored idempotency bodies are read by other stories (SG-301): their
// JSON keys are pinned to the contract names, not to Go's field names.
func TestFrozenQuoteJSONKeys_SG205_AC5(t *testing.T) {
	at := time.Date(2026, 10, 1, 5, 0, 0, 0, time.UTC)
	q := QuoteView{AsOf: at, StayAmount: 1, ExtrasAmount: 2, Total: 3, DepositPaid: 4, BalanceDue: 5, RefundDue: 6, Capped: true,
		Lines: []LineView{{Code: "OVERNIGHT", Quantity: 1, UnitAmount: 2, Amount: 3}}}
	inv := InvoiceView{ID: "iv", StayID: "st", RoomCode: "A1", BillCode: "PH1", Status: "OPEN", CreatedAt: at, Quote: q}
	d := StayDetail{ID: "st", RoomID: "r", RoomCode: "A1", RentalType: "OVERNIGHT", Status: "ACTIVE", CheckInAt: at, CheckOutAt: &at,
		GuestName: "g", GuestPhone: "p", GuestID: GuestIDIndicators{HasIDNumber: true}, Deposit: 1, PricingVersion: 1, Quote: q,
		Extras: []ExtraView{{ServiceCode: "WATER", Name: LocalizedName{VI: "Nuoc", EN: "Water"}, Quantity: 1, UnitAmount: 2, Amount: 2}}}

	rawInv, _ := json.Marshal(inv)
	rawDet, _ := json.Marshal(d)
	var invM, detM map[string]json.RawMessage
	_ = json.Unmarshal(rawInv, &invM)
	_ = json.Unmarshal(rawDet, &detM)

	want := map[string][]string{
		"invoice": {"billCode", "createdAt", "id", "quote", "roomCode", "status", "stayId"},
		"quote":   {"asOf", "balanceDue", "capped", "depositPaid", "extrasAmount", "lines", "refundDue", "stayAmount", "total"},
		"line":    {"amount", "code", "quantity", "unitAmount"},
		"stay": {"checkInAt", "checkOutAt", "deposit", "extras", "guestId", "guestName", "guestPhone", "id", "pricingVersion",
			"quote", "rentalType", "roomCode", "roomId", "status"},
		"extra": {"amount", "name", "quantity", "serviceCode", "unitAmount"},
		"name":  {"en", "vi"},
	}
	var lines, extras []json.RawMessage
	_ = json.Unmarshal(mustField(t, invM["quote"], "lines"), &lines)
	_ = json.Unmarshal(detM["extras"], &extras)
	got := map[string][]string{
		"invoice": keysOf(t, rawInv), "quote": keysOf(t, invM["quote"]), "line": keysOf(t, lines[0]),
		"stay": keysOf(t, rawDet), "extra": keysOf(t, extras[0]), "name": keysOf(t, mustField(t, extras[0], "name")),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v\nwant %v", got, want)
	}

	var backInv InvoiceView
	var backDet StayDetail
	if err := json.Unmarshal(rawInv, &backInv); err != nil || !reflect.DeepEqual(backInv, inv) {
		t.Errorf("invoice round trip: %+v %v", backInv, err)
	}
	if err := json.Unmarshal(rawDet, &backDet); err != nil || !reflect.DeepEqual(backDet, d) {
		t.Errorf("stay round trip: %+v %v", backDet, err)
	}
}

func mustField(t *testing.T, raw json.RawMessage, key string) json.RawMessage {
	t.Helper()
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m[key]
}
