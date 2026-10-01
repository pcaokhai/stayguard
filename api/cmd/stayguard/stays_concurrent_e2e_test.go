//go:build integration

package main

import (
	"sync"
	"testing"
)

const (
	checkInRacers = 10
	raceRounds    = 5
)

func TestCheckInConcurrent_SG203_AC2(t *testing.T) {
	e := newEnv(t)
	a := e.demo("OWNER", "vi", "")
	tenant, token := a.str("tenantId"), a.str("accessToken")
	e.seedStayTenant(tenant, raceRounds+1)

	// Distinct keys, same room: one winner, the rest see ROOM_NOT_VACANT. Fresh room every round.
	for round := 1; round <= raceRounds; round++ {
		statuses, codes := make([]int, checkInRacers), make([]string, checkInRacers)
		var wg sync.WaitGroup
		for i := 0; i < checkInRacers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				st, raw := e.checkIn(token, round, newKey(), stayBody(nil))
				statuses[i] = st
				codes[i], _ = parse(raw)["code"].(string)
			}()
		}
		wg.Wait()
		created, conflicts := 0, 0
		for i, st := range statuses {
			switch {
			case st == 201:
				created++
			case st == 409 && codes[i] == "ROOM_NOT_VACANT":
				conflicts++
			default:
				t.Errorf("round %d: unexpected %d %s", round, st, codes[i])
			}
		}
		if created != 1 || conflicts != checkInRacers-1 {
			t.Errorf("round %d: %d created, %d conflicts", round, created, conflicts)
		}
		if n := e.count(`SELECT count(*) FROM app.stays WHERE unit_id = $1 AND status = 'ACTIVE'`, roomID(round)); n != 1 {
			t.Errorf("round %d: %d active stays", round, n)
		}
		if n := e.count(`SELECT count(*) FROM app.units WHERE id = $1 AND status = 'OCCUPIED'`, roomID(round)); n != 1 {
			t.Errorf("round %d: room not OCCUPIED", round)
		}
	}

	// Same key and body from many goroutines: every caller gets the one stay, byte for byte.
	key, room := newKey(), raceRounds+1
	bodies := make([]string, checkInRacers)
	var wg sync.WaitGroup
	for i := 0; i < checkInRacers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, raw := e.checkIn(token, room, key, stayBody(nil))
			if st != 201 {
				t.Errorf("same-key racer %d: status %d %s", i, st, raw)
			}
			bodies[i] = string(raw)
		}()
	}
	wg.Wait()
	for i, b := range bodies {
		if b != bodies[0] {
			t.Errorf("racer %d body differs: %s vs %s", i, b, bodies[0])
		}
	}
	if n := e.count(`SELECT count(*) FROM app.stays WHERE unit_id = $1`, roomID(room)); n != 1 {
		t.Errorf("%d stay rows for the replayed key", n)
	}
}
