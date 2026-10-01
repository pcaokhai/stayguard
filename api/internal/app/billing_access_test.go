package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// billOps drives both billing commands through one table.
var billOps = map[string]func(e *billEnv, c Caller, stayID, key string) error{
	"addStayExtras": func(e *billEnv, c Caller, id, k string) error {
		_, _, err := e.b.AddExtras(context.Background(), c, id, k, []stay.ExtraInput{{ServiceCode: "WATER", Quantity: 1}})
		return err
	},
	"checkoutStay": func(e *billEnv, c Caller, id, k string) error {
		_, _, err := e.b.Checkout(context.Background(), c, id, k)
		return err
	},
}

func TestStayAccess_SG205_AC2(t *testing.T) {
	for name, op := range billOps {
		e := newBillEnv(t)
		e.seedBillStay(t, StayRecord{})
		e.repo.records[tenantB]["stB"] = StayRecord{ID: "stB", BuildingID: "bB", Status: "ACTIVE"}

		h := e.caller
		h.Role = access.RoleHousekeeping
		if err := op(e, h, "st1", callID(1)); !errors.Is(err, access.ErrRoleForbidden) || e.repo.calls != 0 || e.levels.calls != 0 {
			t.Fatalf("%s role: %v", name, err)
		}
		for _, lv := range []access.Level{access.NONE, access.VIEW} {
			e.setLevel(lv)
			if err := op(e, e.caller, "st1", callID(1)); !errors.Is(err, access.ErrBuildingForbidden) {
				t.Fatalf("%s level %v: %v", name, lv, err)
			}
		}
		e.setLevel(access.EDIT)
		foreign := op(e, e.caller, "stB", callID(2))
		unknown := op(e, e.caller, "stNone", callID(2))
		if !errors.Is(foreign, ErrNotFound) || foreign.Error() != unknown.Error() || !errors.Is(unknown, ErrNotFound) {
			t.Fatalf("%s: foreign %v unknown %v", name, foreign, unknown)
		}
		if e.svc.stock(tenantA, svcWater) != 5 || len(e.idem.m) != 0 || len(e.audit.entries) != 0 {
			t.Fatalf("%s: a refused call changed state", name)
		}
		// A replay is still level-checked: the stored answer is not handed to a caller who lost EDIT.
		if err := op(e, e.caller, "st1", callID(3)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		e.setLevel(access.VIEW)
		if err := op(e, e.caller, "st1", callID(3)); !errors.Is(err, access.ErrBuildingForbidden) {
			t.Fatalf("%s replay: %v", name, err)
		}
	}
}

func TestNoPersonalDataInErrors_SG205_AC2(t *testing.T) {
	const nameMarker, phoneMarker = "NAMEMARKERZ", "PHONEMARKER99"
	e := newBillEnv(t)
	out := e.clock.now
	rec := e.seedBillStay(t, StayRecord{Status: "CHECKED_OUT", CheckOutAt: &out})
	rec.GuestName, rec.GuestPhone = nameMarker, phoneMarker
	e.repo.records[tenantA]["st1"] = rec
	var errs []error
	for i := 0; i < 2; i++ {
		_, _, a := e.b.AddExtras(context.Background(), e.caller, "st1", callID(i), []stay.ExtraInput{{ServiceCode: "WATER", Quantity: 1}})
		errs = append(errs, a)
	}
	_, _, v := e.b.AddExtras(context.Background(), e.caller, "st1", callID(9), []stay.ExtraInput{{ServiceCode: "WATER", Quantity: 0}})
	errs = append(errs, v)
	e.setLevel(access.VIEW)
	_, _, f := e.b.Checkout(context.Background(), e.caller, "st1", callID(5))
	errs = append(errs, f)
	e.setLevel(access.EDIT)
	r2 := e.seedBillStay(t, StayRecord{ID: "st2"})
	r2.GuestName, r2.GuestPhone = nameMarker, phoneMarker
	e.repo.records[tenantA]["st2"] = r2
	_, _, c := e.b.Checkout(context.Background(), e.caller, "st2", callID(6))
	if c != nil {
		t.Fatal(c)
	}
	for _, err := range errs {
		if err == nil || strings.Contains(err.Error(), nameMarker) || strings.Contains(err.Error(), phoneMarker) {
			t.Fatalf("error leaks or is missing: %v", err)
		}
	}
	for _, a := range e.audit.entries {
		if raw := mustJSON(t, a); strings.Contains(raw, nameMarker) || strings.Contains(raw, phoneMarker) {
			t.Fatalf("audit leaks: %s", raw)
		}
	}
	for _, inv := range e.repo.invoices[tenantA] {
		if strings.Contains(string(inv.Quote), nameMarker) {
			t.Fatalf("stored quote leaks: %s", inv.Quote)
		}
	}
}
