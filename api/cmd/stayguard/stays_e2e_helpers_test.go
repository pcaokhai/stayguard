//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// testDataKey is a throwaway AES-256 key; no real key is ever in the repository.
var testDataKey = bytes.Repeat([]byte{0x42}, 32)

const (
	// Markers are unique strings the privacy tests search for in logs, rows and responses.
	idMarker     = "IDMARK98765"
	nameMarker   = "Zzyxq Wvutsr"
	phoneMarker  = "0977123456"
	stayBuilding = "bld_st"
)

// syncBuffer is a log sink safe for the concurrent handlers of the test server.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *syncBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

// seedStayTenant gives a tenant one building with n VACANT rooms ("<bld>_r1".."<bld>_rN") whose
// unit type carries a valid rate plan.
func (e *env) seedStayTenant(tenant string, n int) {
	e.t.Helper()
	e.seedBuildings(tenant, stayBuilding)
	e.exec(`INSERT INTO app.floors (id, tenant_id, building_id, level) VALUES ($1, $2, $3, 1)`, stayBuilding+"_f", tenant, stayBuilding)
	e.exec(`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version)
		VALUES ($1, $2, 'STD', '{"vi":"Tieu chuan","en":"Standard"}', $3, 1)`, stayBuilding+"_ut", tenant, seedPlanSnapshot())
	e.exec(`INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status, attributes)
		SELECT $1 || '_r' || k, $2, $1, $1 || '_f', $1 || '_ut', 'R' || k, 'VACANT', '{}' FROM generate_series(1, $3::int) k`,
		stayBuilding, tenant, n)
}

func roomID(n int) string { return fmt.Sprintf("%s_r%d", stayBuilding, n) }

// newKey is a random version 4 UUID, the shape the contract demands for Idempotency-Key.
func newKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// stayBody is a valid createStay body; tests override single fields.
func stayBody(over map[string]any) map[string]any {
	b := map[string]any{"rentalType": "OVERNIGHT", "guestName": "Nguyen An", "guestPhone": "0900000001", "deposit": 100000}
	for k, v := range over {
		b[k] = v
	}
	return b
}

// send returns the status and the raw response bytes, for byte-for-byte replay checks. A missing
// key is the empty string.
func (e *env) send(method, path, token, key string, body any) (int, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Errorf("%s %s: %v", method, path, err) // not Fatal: callers may be goroutines
		return 0, nil
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

func (e *env) checkIn(token string, room int, key string, body any) (int, []byte) {
	return e.send("POST", "/v1/rooms/"+roomID(room)+"/stays", token, key, body)
}

func parse(raw []byte) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// dumpTable returns every row of an app table as text, to search for data that must not be there.
func (e *env) dumpTable(table string) string {
	e.t.Helper()
	rows, err := e.owner.Query(context.Background(), fmt.Sprintf(`SELECT t::text FROM app.%s t`, table))
	if err != nil {
		e.t.Fatalf("dump %s: %v", table, err)
	}
	defer rows.Close()
	var sb strings.Builder
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			e.t.Fatalf("scan %s: %v", table, err)
		}
		sb.WriteString(s + "\n")
	}
	return sb.String()
}

// fixedCheckIn is 21:30 in Ho Chi Minh City, inside the seeded overnight window.
var fixedCheckIn = time.Date(2026, 10, 1, 14, 30, 0, 0, time.UTC)
