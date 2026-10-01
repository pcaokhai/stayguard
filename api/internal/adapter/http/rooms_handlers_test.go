package httpadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %q", rec.Body.String())
	}
	return m
}

func TestListBuildingsHTTP_SG201_AC1(t *testing.T) {
	f := &fakeRooms{buildings: []app.BuildingView{{
		ID: "b1", Code: "B-A", Name: "Block A", Level: access.EDIT,
		Counts: room.Counts{Vacant: 1, Occupied: 2, Overdue: 3, ToClean: 4, Maintenance: 5},
	}}}
	h := roomRouter(f, true)
	rec := getAuthed(h, "/v1/buildings")
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	items := decode(t, rec)["items"].([]any)
	b := items[0].(map[string]any)
	want := `{"code":"B-A","counts":{"maintenance":5,"occupied":2,"overdue":3,"toClean":4,"vacant":1},"id":"b1","level":"EDIT","name":"Block A"}`
	got, _ := json.Marshal(b)
	if string(got) != want {
		t.Errorf("building = %s\nwant       %s", got, want)
	}
	if f.caller.Role != access.RoleReceptionist || f.caller.TenantID != "tn_a" {
		t.Errorf("caller not passed through: %+v", f.caller)
	}
	f.buildings = nil
	if rec = getAuthed(h, "/v1/buildings"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("empty list must be [] not null: %d %s", rec.Code, rec.Body)
	}
	if rec = do(h, "GET", "/v1/buildings", ""); rec.Code != 401 {
		t.Errorf("no token: code=%d", rec.Code)
	}
}

func sampleRoom(withStay bool) app.RoomView {
	r := app.RoomView{
		ID: "r1", Code: "A101", BuildingID: "b1", Floor: 2, UnitTypeCode: "VIP",
		UnitTypeName: app.LocalizedName{VI: "Cao cap", EN: "VIP"}, Status: room.StatusVacant,
	}
	if withStay {
		r.Status = room.StatusOccupied
		r.ActiveStay = &app.StayView{
			ID: "s1", RentalType: room.Overnight, GuestName: "Guest",
			CheckInAt: time.Date(2026, 10, 1, 1, 30, 0, 0, time.UTC), ElapsedMinutes: 90, RunningTotal: 350000,
		}
	}
	return r
}

func TestRoomsHTTP_SG201_AC2(t *testing.T) {
	note := "sea view"
	occupied := sampleRoom(true)
	vacant := sampleRoom(false)
	vacant.Note = &note
	f := &fakeRooms{rooms: []app.RoomView{occupied, vacant}, one: occupied}
	h := roomRouter(f, true)

	rec := getAuthed(h, "/v1/buildings/b1/rooms?status=OVERDUE")
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	if f.buildID != "b1" || f.status == nil || *f.status != room.StatusOverdue {
		t.Errorf("args: building=%q status=%v", f.buildID, f.status)
	}
	items := decode(t, rec)["items"].([]any)
	gotOcc, _ := json.Marshal(items[0])
	wantOcc := `{"activeStay":{"checkInAt":"2026-10-01T01:30:00Z","elapsedMinutes":90,"guestName":"Guest","id":"s1","rentalType":"OVERNIGHT","runningTotal":350000},"buildingId":"b1","code":"A101","floor":2,"id":"r1","status":"OCCUPIED","unitType":{"code":"VIP","name":{"en":"VIP","vi":"Cao cap"}}}`
	if string(gotOcc) != wantOcc {
		t.Errorf("occupied = %s\nwant       %s", gotOcc, wantOcc)
	}
	if v := items[1].(map[string]any); v["note"] != "sea view" || v["activeStay"] != nil {
		t.Errorf("vacant room: %v", v)
	}

	getAuthed(h, "/v1/buildings/b1/rooms")
	if f.status != nil {
		t.Errorf("no status param must pass nil, got %v", *f.status)
	}
	calls := f.callsCnt
	if rec = getAuthed(h, "/v1/buildings/b1/rooms?status=BOGUS"); rec.Code != 400 || f.callsCnt != calls {
		t.Errorf("unknown status: code=%d calls %d->%d", rec.Code, calls, f.callsCnt)
	}

	rec = getAuthed(h, "/v1/rooms/r1")
	if rec.Code != 200 || f.roomID != "r1" {
		t.Fatalf("getRoom code=%d id=%q", rec.Code, f.roomID)
	}
	if got := decode(t, rec); got["id"] != "r1" || got["activeStay"] == nil {
		t.Errorf("getRoom body: %v", got)
	}
}

func TestProblemMapping_SG201_AC4(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{access.ErrRoleForbidden, 403, "ROLE_FORBIDDEN"},
		{access.ErrBuildingForbidden, 403, "BUILDING_FORBIDDEN"},
		{app.ErrNotFound, 404, "NOT_FOUND"},
		{app.ErrPricingUnavailable, 503, "PRICING_UNAVAILABLE"},
		{errors.New("room: rate plan snapshot: MARKER-LEAK"), 500, "INTERNAL"},
	}
	for _, c := range cases {
		wrapped := errors.Join(errors.New("wrapped MARKER-LEAK"), c.err)
		h := roomRouter(&fakeRooms{err: wrapped}, true)
		for _, path := range []string{"/v1/buildings", "/v1/buildings/b1/rooms", "/v1/rooms/r1"} {
			rec := getAuthed(h, path)
			if status, code := problemOf(t, rec); rec.Code != c.status || status != c.status || code != c.code {
				t.Errorf("%v %s: code=%d problem=%d/%s", c.err, path, rec.Code, status, code)
			}
			if rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Errorf("%s content-type %q", path, rec.Header().Get("Content-Type"))
			}
			if strings.Contains(rec.Body.String(), "MARKER-LEAK") {
				t.Errorf("%s leaked error text: %s", path, rec.Body)
			}
		}
	}
}

func TestProblemMappingNotFoundIdentical_SG201_AC4(t *testing.T) {
	h := roomRouter(&fakeRooms{err: app.ErrNotFound}, true)
	a, b := getAuthed(h, "/v1/rooms/foreign-id"), getAuthed(h, "/v1/rooms/random-id")
	if a.Code != 404 || a.Body.String() != b.Body.String() {
		t.Errorf("bodies differ: %q vs %q", a.Body, b.Body)
	}
}

func TestRoomMapUnknownErrorLogged_SG201_AC4(t *testing.T) {
	var buf bytes.Buffer
	h := NewRouter(slog.New(slog.NewJSONHandler(&buf, nil)), Options{
		Probe: readyProbe{}, Sessions: &fakeSessions{}, RoomMapEnabled: true,
		Rooms: &fakeRooms{err: errors.New("room: bad snapshot MARKER-LEAK")},
	})
	if rec := getAuthed(h, "/v1/rooms/r1"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(buf.String(), "MARKER-LEAK") {
		t.Errorf("error not logged server-side: %s", buf.String())
	}
}

func TestRoomMapFlagOff_SG201_AC1(t *testing.T) {
	f := &fakeRooms{forbid: t}
	h := roomRouter(f, false)
	for _, path := range []string{"/v1/buildings", "/v1/buildings/b1/rooms", "/v1/rooms/r1"} {
		rec := getAuthed(h, path)
		if status, code := problemOf(t, rec); rec.Code != 404 || status != 404 || code != "FEATURE_DISABLED" {
			t.Errorf("%s: code=%d problem=%d/%s", path, rec.Code, status, code)
		}
	}
	if f.callsCnt != 0 {
		t.Errorf("use case called %d times", f.callsCnt)
	}
}
