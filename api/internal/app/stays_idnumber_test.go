package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

func TestIdNumberStored_SG203_AC4(t *testing.T) {
	e := newStayEnv(t)
	v, _, err := create(e, "k1", goodInput())
	if err != nil {
		t.Fatal(err)
	}
	ns := e.repo.inserted[0]
	if !bytes.HasPrefix(ns.IDNumberEnc, []byte(encMark)) || bytes.Equal(ns.IDNumberEnc, []byte(idMarker)) {
		t.Fatalf("stored value is not ciphertext: %s", ns.IDNumberEnc)
	}
	if v.IDNumberMasked == nil || *v.IDNumberMasked != "*****345" {
		t.Fatalf("masked: %v", v.IDNumberMasked)
	}
	body, _ := json.Marshal(v)
	if strings.Contains(string(body), idMarker) {
		t.Fatal("response holds the plain id number")
	}
	// The hash input holds a keyed fingerprint, not the plain id: it equals the expected body hash
	// and differs from the hash of a body that carries the id itself.
	fp := hex.EncodeToString(e.enc.Fingerprint(tenantA, "stays.id_number", []byte(idMarker)))
	want := RequestHash([]byte(`{"rentalType":"HOURLY","guestName":"Marker Guest","guestPhone":"+84 900 000 111","idNumberFingerprint":"` + fp + `","deposit":200000}`))
	plain := RequestHash([]byte(`{"rentalType":"HOURLY","guestName":"Marker Guest","guestPhone":"+84 900 000 111","idNumberFingerprint":"` + idMarker + `","deposit":200000}`))
	if got := e.idem.hashes[0]; got != want || got == plain {
		t.Fatalf("hash %s want %s", got, want)
	}
	if _, _, err := create(newStayEnv(t), "k", CreateStayInput{RentalType: "HOURLY", GuestName: "A", GuestPhone: "123456"}); err != nil {
		t.Fatalf("no id number is allowed: %v", err)
	}
}

func TestIdNumberBoundToStay_SG203_AC4(t *testing.T) {
	e := newStayEnv(t)
	e.repo.rooms[tenantA]["rm2"] = CheckInRoom{ID: "rm2", Code: "102", BuildingID: bldgA, StoredStatus: "VACANT", RatePlan: stayPlan().Snapshot()}
	first, _, err := create(e, "k1", goodInput())
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := e.s.CreateStay(context.Background(), e.caller, "rm2", "k2", goodInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.GetStay(context.Background(), e.caller, first.ID); err != nil {
		t.Fatal(err)
	}
	moved := e.repo.records[tenantA][second.ID]
	moved.IDNumberEnc = e.repo.records[tenantA][first.ID].IDNumberEnc // copied to another stay's row
	e.repo.records[tenantA][second.ID] = moved
	_, err = e.s.GetStay(context.Background(), e.caller, second.ID)
	if err == nil || strings.Contains(err.Error(), idMarker) {
		t.Fatalf("a ciphertext moved to another stay must fail to open: %v", err)
	}
}

func TestIdNumberFailsClosed_SG203_AC4(t *testing.T) {
	e := newStayEnv(t)
	e.enc.encryptErr = errors.New("boom")
	e.s.enc = e.enc
	if _, _, err := create(e, "k1", goodInput()); err == nil || len(e.repo.inserted) != 0 {
		t.Fatalf("encrypt failure must stop check-in: %v", err)
	}
	e = newStayEnv(t)
	v, _, _ := create(e, "k1", goodInput())
	e.enc.decryptErr = errors.New("boom")
	e.s.enc = e.enc
	if got, err := e.s.GetStay(context.Background(), e.caller, v.ID); err == nil || got.IDNumberMasked != nil {
		t.Fatalf("decrypt failure must be an error: %v %+v", err, got)
	}
}

func TestNoPersonalDataInErrors_SG203_AC4(t *testing.T) {
	const name, phone = "MarkerName", "+84 MARKERPHONE"
	id := "ID-MARKER"
	bad := CreateStayInput{RentalType: "WEEKLY-MARKER", GuestName: name, GuestPhone: phone, IDNumber: &id, Deposit: -5}
	e := newStayEnv(t)
	errs := []error{}
	add := func(_ StayDetail, _ bool, err error) { errs = append(errs, err) }
	add(create(e, "k1", bad))
	ok := goodInput()
	ok.GuestName, ok.GuestPhone = name, "+84 111 222 333"
	idOK := "MARKERID99"
	ok.IDNumber = &idOK
	e.setStatus(room.StatusOccupied)
	add(create(e, "k2", ok))
	e.setStatus(room.StatusVacant)
	add(create(e, "k3", ok))
	ok.Deposit = 1
	add(create(e, "k3", ok)) // key reused
	e.repo.markErr = errors.New("db down")
	add(create(newStayEnvWith(t, e), "k4", ok))
	k := newStayEnv(t)
	k.enc.encryptErr = errors.New("kms down")
	k.s.enc = k.enc
	ok.Deposit = 2
	_, _, kmsErr := create(k, "k5", ok)
	if kmsErr == nil || len(k.repo.inserted) != 0 {
		t.Fatalf("encrypt failure must stop check-in: %v", kmsErr)
	}
	add(StayDetail{}, false, kmsErr)
	failed := 0
	for _, err := range errs {
		if err == nil { // k3 succeeds once: the room was freed again
			continue
		}
		failed++
		for _, m := range []string{name, phone, "MARKERPHONE", id, idOK, "WEEKLY-MARKER", "FAKECT"} {
			if strings.Contains(err.Error(), m) {
				t.Fatalf("error leaks %q: %v", m, err)
			}
		}
	}
	if failed != 5 {
		t.Fatalf("expected 5 failing cases, got %d", failed)
	}
}

// newStayEnvWith returns a fresh env whose repo fails the occupy step like e's does.
func newStayEnvWith(t *testing.T, e *stayEnv) *stayEnv {
	n := newStayEnv(t)
	n.repo.markErr = e.repo.markErr
	return n
}
