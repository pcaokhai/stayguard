package app

import (
	"context"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

type fakeLevels struct {
	levels map[string]access.Level
	calls  int
}

func (f *fakeLevels) Levels(context.Context, Caller, []string) (map[string]access.Level, error) {
	f.calls++
	return f.levels, nil
}

// fakeRoomRepo scopes by the tenant of the tx, like the real repo.
type fakeRoomRepo struct {
	buildings map[string][]BuildingRow
	rooms     map[string][]RoomRow
	tz        string
	calls     int
}

func (r *fakeRoomRepo) Buildings(_ context.Context, tx Tx) ([]BuildingRow, error) {
	r.calls++
	return r.buildings[tx.TenantID()], nil
}

func (r *fakeRoomRepo) Rooms(_ context.Context, tx Tx, f RoomFilter) ([]RoomRow, error) {
	r.calls++
	var out []RoomRow
	for _, row := range r.rooms[tx.TenantID()] {
		if (f.BuildingID == "" || row.BuildingID == f.BuildingID) && (f.RoomID == "" || row.ID == f.RoomID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeRoomRepo) Timezone(context.Context, Tx) (string, error) {
	r.calls++
	return r.tz, nil
}

type quoteCall struct {
	snapshot string
	rt       room.RentalType
	checkIn  time.Time
	now      time.Time
	zone     string
}

type fakeQuoter struct {
	total int64
	err   error
	calls []quoteCall
}

func (q *fakeQuoter) RunningTotal(_ context.Context, snap []byte, rt room.RentalType, in, now time.Time, loc *time.Location) (int64, error) {
	q.calls = append(q.calls, quoteCall{string(snap), rt, in, now, loc.String()})
	return q.total, q.err
}
