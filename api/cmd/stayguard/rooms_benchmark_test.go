//go:build integration

package main

import (
	"sort"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/permissions"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/adapter/pricing"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	benchRooms     = 200
	benchFloors    = 4
	benchOccupied  = benchRooms * 40 / 100
	benchWarmup    = 20
	benchRuns      = 300
	benchP95Budget = 150 * time.Millisecond
)

// seedBenchBuilding adds 200 units on 4 floors; the first 80 get an ACTIVE overnight stay, half of
// them checked in two days ago (past the window end, so OVERDUE) and half one hour ago.
func (e *env) seedBenchBuilding(tenant, bld string) {
	e.t.Helper()
	e.seedBuildings(tenant, bld)
	e.exec(`INSERT INTO app.floors (id, tenant_id, building_id, level)
		SELECT $2 || '_f' || n, $1, $2, n FROM generate_series(1, $3::int) n`, tenant, bld, benchFloors)
	e.exec(`INSERT INTO app.unit_types (id, tenant_id, code, name, rate_plan, rate_plan_version)
		VALUES ($1, $2, 'STD', '{"vi":"Tieu chuan","en":"Standard"}', '{}', 1)`, bld+"_ut", tenant)
	e.exec(`INSERT INTO app.units (id, tenant_id, building_id, floor_id, unit_type_id, code, status, attributes)
		SELECT $2 || '_u' || n, $1, $2, $2 || '_f' || (n % $3::int + 1), $2 || '_ut', 'R' || n,
			CASE WHEN n <= $4::int THEN 'OCCUPIED' ELSE 'VACANT' END, '{}'
		FROM generate_series(1, $5::int) n`, tenant, bld, benchFloors, benchOccupied, benchRooms)
	e.exec(`INSERT INTO app.stays (id, tenant_id, unit_id, rental_type, status, guest_name, guest_phone, check_in_at, rate_plan_snapshot, rate_plan_schema)
		SELECT $2 || '_s' || n, $1, $2 || '_u' || n, 'OVERNIGHT', 'ACTIVE', 'Guest', '0900000000',
			CASE WHEN n % 2 = 0 THEN $3::timestamptz - interval '48 hours' ELSE $3::timestamptz - interval '1 hour' END,
			$5, 1
		FROM generate_series(1, $4::int) n`, tenant, bld, e.start, benchOccupied, seedPlanSnapshot())
}

// measure runs warmup then runs sequential GETs and returns p50 and p95 of the server time.
func (e *env) measure(path, token string) (p50, p95 time.Duration, last reply) {
	e.t.Helper()
	for i := 0; i < benchWarmup; i++ {
		e.call("GET", path, token, nil)
	}
	d := make([]time.Duration, benchRuns)
	for i := range d {
		begin := time.Now()
		last = e.call("GET", path, token, nil)
		d[i] = time.Since(begin)
		if last.status != 200 {
			e.t.Fatalf("GET %s: status %d body %v", path, last.status, last.body)
		}
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[benchRuns/2-1], d[benchRuns*95/100-1], last
}

func TestListRoomsBenchmark_SG201_AC5(t *testing.T) {
	e := newEnvRooms(t, func(uow app.UnitOfWork, clk app.Clock) *app.Rooms {
		return app.NewRooms(uow, postgres.RoomRepo{}, permissions.RoleBased{}, pricing.Quoter{}, clk)
	})
	owner := e.demo("OWNER", "vi", "")
	e.seedBenchBuilding(owner.str("tenantId"), "bld_bench")
	token := owner.str("accessToken")

	p50, p95, res := e.measure("/v1/buildings/bld_bench/rooms", token)
	if items, _ := res.body["items"].([]any); len(items) != benchRooms {
		t.Fatalf("listRooms returned %d rooms, want %d", len(items), benchRooms)
	}
	t.Logf("listRooms (%d rooms, %d runs): p50=%v p95=%v", benchRooms, benchRuns, p50, p95)
	if p95 > benchP95Budget {
		t.Errorf("listRooms p95 = %v, budget %v", p95, benchP95Budget)
	}

	b50, b95, _ := e.measure("/v1/buildings", token)
	t.Logf("listBuildings (counters): p50=%v p95=%v", b50, b95)
}
