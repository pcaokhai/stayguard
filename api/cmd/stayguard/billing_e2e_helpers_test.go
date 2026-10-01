//go:build integration

package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	waterPrice = 10_000
	beerPrice  = 25_000
	// billDay is fixedCheckIn plus an hour: 22:30 on 1 October in Ho Chi Minh City, so bill codes carry 1001.
	billDay = time.Hour
)

// seedServices gives a tenant WATER and BEER with the given stock; ids are prefixed with the tenant.
func (e *env) seedServices(tenant string, stock int) {
	e.t.Helper()
	e.exec(`INSERT INTO app.services (id, tenant_id, code, name, price, stock) VALUES
		($1 || '_water', $1, 'WATER', '{"vi":"Nuoc","en":"Water"}', $2, $4),
		($1 || '_beer', $1, 'BEER', '{"vi":"Bia","en":"Beer"}', $3, $4)`, tenant, waterPrice, beerPrice, stock)
}

func (e *env) stock(tenant, code string) int {
	e.t.Helper()
	return e.count(`SELECT stock FROM app.services WHERE tenant_id = $1 AND code = $2`, tenant, code)
}

func extrasBody(lines ...any) map[string]any {
	items := []map[string]any{}
	for i := 0; i+1 < len(lines); i += 2 {
		items = append(items, map[string]any{"serviceCode": lines[i], "quantity": lines[i+1]})
	}
	return map[string]any{"items": items}
}

func (e *env) addExtras(token, stayID, key string, body any) (int, []byte) {
	return e.send("POST", "/v1/stays/"+stayID+"/extras", token, key, body)
}

func (e *env) checkout(token, stayID, key string) (int, []byte) {
	return e.send("POST", "/v1/stays/"+stayID+"/checkout", token, key, nil)
}

// openStay checks a guest in at the fixed clock and returns the stay id.
func (e *env) openStay(token string, room int, over map[string]any) string {
	e.t.Helper()
	e.clock.set(fixedCheckIn)
	st, raw := e.checkIn(token, room, newKey(), stayBody(over))
	id, _ := parse(raw)["id"].(string)
	if st != 201 || id == "" {
		e.t.Fatalf("check-in room %d: %d %s", room, st, raw)
	}
	return id
}

// race runs n copies of fn at once and waits; i is the racer number.
func race(n int, fn func(i int)) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(i)
		}()
	}
	wg.Wait()
}

func problemCode(raw []byte) string {
	c, _ := parse(raw)["code"].(string)
	return c
}

func describe(st int, raw []byte) string { return fmt.Sprintf("%d %s", st, raw) }

func containsLevel(logs, level string) bool {
	return strings.Contains(logs, `"level":"`+level+`"`)
}
