package httpadapter

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
)

// fakeRooms returns canned views and records the caller and arguments of the last call. When forbid
// is set any call fails the test, proving a request was rejected before the use case.
type fakeRooms struct {
	forbid    *testing.T
	err       error
	buildings []app.BuildingView
	rooms     []app.RoomView
	one       app.RoomView

	caller   app.Caller
	buildID  string
	roomID   string
	status   *room.Status
	callsCnt int
}

func (f *fakeRooms) touch(c app.Caller) {
	f.callsCnt++
	f.caller = c
	if f.forbid != nil {
		f.forbid.Errorf("rooms use case called although the request must be rejected earlier")
	}
}

func (f *fakeRooms) ListBuildings(_ context.Context, c app.Caller) ([]app.BuildingView, error) {
	f.touch(c)
	return f.buildings, f.err
}

func (f *fakeRooms) ListRooms(_ context.Context, c app.Caller, buildingID string, s *room.Status) ([]app.RoomView, error) {
	f.touch(c)
	f.buildID, f.status = buildingID, s
	return f.rooms, f.err
}

func (f *fakeRooms) GetRoom(_ context.Context, c app.Caller, roomID string) (app.RoomView, error) {
	f.touch(c)
	f.roomID = roomID
	return f.one, f.err
}

func roomRouter(f *fakeRooms, enabled bool) http.Handler {
	return NewRouter(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), Options{
		Probe: readyProbe{}, Sessions: &fakeSessions{}, Rooms: f, RoomMapEnabled: enabled,
	})
}

func getAuthed(h http.Handler, path string) *httptest.ResponseRecorder {
	return do(h, "GET", path, "Bearer "+goodToken)
}
