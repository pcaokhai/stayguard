package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

func (e *billEnv) addExtras(key string, items ...stay.ExtraInput) (StayDetail, bool, error) {
	return e.b.AddExtras(context.Background(), e.caller, "st1", key, items)
}

func TestListServices_SG205_AC1(t *testing.T) {
	e := newBillEnv(t)
	for _, role := range []access.Role{access.RoleOwner, access.RoleReceptionist} {
		c := e.caller
		c.Role = role
		got, err := e.b.ListServices(context.Background(), c)
		want := []Service{
			{Code: "BEER", Name: LocalizedName{VI: "Bia", EN: "Beer"}, Price: 25_000, Stock: 3},
			{Code: "WATER", Name: LocalizedName{VI: "Nuoc", EN: "Water"}, Price: 15_000, Stock: 5},
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v %+v", role, err, got)
		}
	}
	calls := e.svc.calls
	h := e.caller
	h.Role = access.RoleHousekeeping
	if _, err := e.b.ListServices(context.Background(), h); !errors.Is(err, access.ErrRoleForbidden) || e.svc.calls != calls {
		t.Fatalf("housekeeping: %v", err)
	}
}

func TestAddExtras_SG205_AC2(t *testing.T) {
	e := newBillEnv(t)
	rec := e.seedBillStay(t, StayRecord{Deposit: 10_000})
	e.clock.now = rec.CheckInAt.Add(2 * 60 * 60 * 1e9)
	v, replayed, err := e.addExtras(callID(1), stay.ExtraInput{ServiceCode: "WATER", Quantity: 2}, stay.ExtraInput{ServiceCode: "BEER", Quantity: 1},
		stay.ExtraInput{ServiceCode: "WATER", Quantity: 1})
	if err != nil || replayed {
		t.Fatalf("err=%v replayed=%v", err, replayed)
	}
	if e.svc.stock(tenantA, svcWater) != 2 || e.svc.stock(tenantA, svcBeer) != 2 {
		t.Fatalf("stock must drop once by the merged quantity: %d %d", e.svc.stock(tenantA, svcWater), e.svc.stock(tenantA, svcBeer))
	}
	if !reflect.DeepEqual(e.svc.decOrder, []string{"BEER", "WATER"}) {
		t.Fatalf("stock rows must be taken in code order: %v", e.svc.decOrder)
	}
	if len(v.Extras) != 2 || v.Extras[0].ServiceCode != "BEER" || v.Extras[0].Amount != 25_000 ||
		v.Extras[1].ServiceCode != "WATER" || v.Extras[1].Quantity != 3 || v.Extras[1].UnitAmount != 15_000 || v.Extras[1].Amount != 45_000 {
		t.Fatalf("extras: %+v", v.Extras)
	}
	if v.Quote.ExtrasAmount != 70_000 || v.Quote.Total != v.Quote.StayAmount+70_000 || !v.Quote.AsOf.Equal(e.clock.now) {
		t.Fatalf("quote: %+v", v.Quote)
	}
	if want := expectedDetail(t, e.repo.records[tenantA]["st1"], e.clock.now); !reflect.DeepEqual(v.Quote, want) {
		t.Fatalf("quote differs from an independent computation:\n%+v\n%+v", v.Quote, want)
	}
	if len(e.audit.entries) != 1 {
		t.Fatalf("audit entries: %d", len(e.audit.entries))
	}
	a := e.audit.entries[0]
	var after struct {
		StayID string
		Items  []struct {
			ServiceID              string
			Quantity, Unit, Amount int64
		}
	}
	if a.Action != "stay.extras_added" || a.EntityType != "stay" || a.EntityID != "st1" || a.ActorID != "u1" ||
		json.Unmarshal(a.After, &after) != nil || after.StayID != "st1" || len(after.Items) != 2 ||
		after.Items[1].ServiceID != svcWater || after.Items[1].Quantity != 3 || after.Items[1].Unit != 15_000 || after.Items[1].Amount != 45_000 {
		t.Fatalf("audit: %+v %s", a, a.After)
	}
	for _, banned := range []string{"Water", "Nuoc", "\"G\""} {
		if strings.Contains(string(a.After), banned) {
			t.Fatalf("audit holds %q: %s", banned, a.After)
		}
	}
}

func TestAddExtrasInsufficientStock_SG205_AC1(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{})
	_, _, err := e.addExtras(callID(1), stay.ExtraInput{ServiceCode: "BEER", Quantity: 1}, stay.ExtraInput{ServiceCode: "WATER", Quantity: 6})
	if !errors.Is(err, stay.ErrInsufficientStock) {
		t.Fatalf("err = %v", err)
	}
	if e.svc.stock(tenantA, svcBeer) != 3 || e.svc.stock(tenantA, svcWater) != 5 {
		t.Fatal("the earlier decrement must be rolled back with the failed unit of work")
	}
	if len(e.repo.records[tenantA]["st1"].Extras) != 0 || len(e.audit.entries) != 0 || len(e.idem.m) != 0 {
		t.Fatal("nothing may be persisted")
	}
	// The key left no trace: the same key works once stock allows it.
	if _, replayed, err := e.addExtras(callID(1), stay.ExtraInput{ServiceCode: "WATER", Quantity: 5}); err != nil || replayed {
		t.Fatalf("retry: %v %v", err, replayed)
	}
}

func TestAddExtrasIdempotent_SG205_AC2(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{})
	retryID := callID(7)
	first, _, err := e.addExtras(retryID, stay.ExtraInput{ServiceCode: "WATER", Quantity: 3})
	if err != nil {
		t.Fatal(err)
	}
	// One line of 3 and two lines of 1 + 2 are the same merged request.
	again, replayed, err := e.addExtras(retryID, stay.ExtraInput{ServiceCode: "WATER", Quantity: 1}, stay.ExtraInput{ServiceCode: "WATER", Quantity: 2})
	if err != nil || !replayed || !reflect.DeepEqual(first, again) {
		t.Fatalf("replay: %v %v", err, replayed)
	}
	if e.svc.stock(tenantA, svcWater) != 2 || len(e.repo.records[tenantA]["st1"].Extras) != 1 || len(e.audit.entries) != 1 {
		t.Fatal("a replay must not change stock, rows or audit")
	}
	if _, _, err := e.addExtras(retryID, stay.ExtraInput{ServiceCode: "WATER", Quantity: 4}); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("different body: %v", err)
	}
	if e.svc.stock(tenantA, svcWater) != 2 {
		t.Fatal("a reused key must not change stock")
	}
	hashes := len(e.idem.hashes)
	calls := e.repo.calls + e.svc.calls
	for _, k := range []string{"", strings.Repeat("k", 65)} {
		if _, _, err := e.addExtras(k, stay.ExtraInput{ServiceCode: "WATER", Quantity: 1}); !errors.Is(err, ErrInvalidIdempotencyKey) {
			t.Fatalf("key %d bytes: %v", len(k), err)
		}
	}
	if len(e.idem.hashes) != hashes || e.repo.calls+e.svc.calls != calls {
		t.Fatal("bad keys must be rejected before any I/O")
	}
}

func TestExtrasStayNotActive_SG205_AC4(t *testing.T) {
	e := newBillEnv(t)
	out := e.clock.now
	e.seedBillStay(t, StayRecord{Status: "CHECKED_OUT", CheckOutAt: &out})
	_, _, err := e.addExtras(callID(1), stay.ExtraInput{ServiceCode: "WATER", Quantity: 1})
	if !errors.Is(err, stay.ErrNotActive) || e.svc.stock(tenantA, svcWater) != 5 || len(e.idem.m) != 0 {
		t.Fatalf("err=%v", err)
	}
}

func TestExtrasUnknownService_SG205_AC2(t *testing.T) {
	e := newBillEnv(t)
	e.seedBillStay(t, StayRecord{})
	_, _, err := e.addExtras(callID(1), stay.ExtraInput{ServiceCode: "WATER", Quantity: 1}, stay.ExtraInput{ServiceCode: "GHOSTMARKER", Quantity: 1})
	var ve *stay.ValidationError
	if !errors.As(err, &ve) || !reflect.DeepEqual(ve.Errors, []stay.FieldError{{Path: "items[1].serviceCode", Code: "UNKNOWN"}}) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "GHOSTMARKER") || e.svc.stock(tenantA, svcWater) != 5 {
		t.Fatalf("echo or partial write: %v", err)
	}
	// Input is validated before any I/O.
	calls := e.repo.calls
	if _, _, err := e.addExtras(callID(2)); !errors.As(err, &ve) || e.repo.calls != calls {
		t.Fatalf("empty list: %v", err)
	}
}
