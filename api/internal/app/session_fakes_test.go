package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

type fakeTx struct{ tenant string }

func (t fakeTx) TenantID() string { return t.tenant }

type fakeUoW struct{ tenants []string }

func (u *fakeUoW) Do(ctx context.Context, tenantID string, fn func(context.Context, Tx) error) error {
	u.tenants = append(u.tenants, tenantID)
	return fn(ctx, fakeTx{tenantID})
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type seqIDs struct{ n int }

func (s *seqIDs) New(prefix string) string { s.n++; return fmt.Sprintf("%s_gen%d", prefix, s.n) }

type fixedToken struct{ tok string }

func (f fixedToken) New() (string, error) { return f.tok, nil }

type fakeResolver struct {
	ref    SessionRef
	err    error
	hashes []string
}

func (r *fakeResolver) Resolve(_ context.Context, h string) (SessionRef, error) {
	r.hashes = append(r.hashes, h)
	return r.ref, r.err
}

type sessionRow struct {
	hash, userID string
	expires      time.Time
}

type fakeRepo struct {
	calls         int
	trials        map[string]bool // id -> usable (found by TrialTenant)
	createdTenant []string
	users         map[access.Role]User
	usersByID     map[string]User
	createdUsers  []User
	conflictOnce  bool
	sessions      []sessionRow
	info          TenantInfo
	buildings     []string
	locale        string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{trials: map[string]bool{}, users: map[access.Role]User{}, usersByID: map[string]User{}}
}

func (r *fakeRepo) CreateTrialTenant(_ context.Context, _ Tx, id, _ string, _ time.Time) error {
	r.calls++
	r.createdTenant = append(r.createdTenant, id)
	return nil
}

func (r *fakeRepo) TrialTenant(_ context.Context, _ Tx, id string, _ time.Time) (bool, error) {
	r.calls++
	return r.trials[id], nil
}

func (r *fakeRepo) UserByRole(_ context.Context, _ Tx, role access.Role) (User, bool, error) {
	r.calls++
	u, ok := r.users[role]
	return u, ok, nil
}

func (r *fakeRepo) CreateUser(_ context.Context, _ Tx, id, name string, role access.Role, locale string) (User, error) {
	r.calls++
	if r.conflictOnce {
		r.conflictOnce = false
		r.users[role] = User{ID: "us_winner", Name: name, Role: role, Locale: locale}
		return User{}, ErrConflict
	}
	u := User{ID: id, Name: name, Role: role, Locale: locale}
	r.users[role] = u
	r.createdUsers = append(r.createdUsers, u)
	return u, nil
}

func (r *fakeRepo) InsertSession(_ context.Context, _ Tx, hash, userID string, exp time.Time) error {
	r.calls++
	r.sessions = append(r.sessions, sessionRow{hash, userID, exp})
	return nil
}

func (r *fakeRepo) UserByID(_ context.Context, _ Tx, id string) (User, bool, error) {
	r.calls++
	u, ok := r.usersByID[id]
	return u, ok, nil
}

func (r *fakeRepo) TenantInfo(context.Context, Tx) (TenantInfo, error) { r.calls++; return r.info, nil }

func (r *fakeRepo) BuildingIDs(context.Context, Tx) ([]string, error) {
	r.calls++
	return r.buildings, nil
}

func (r *fakeRepo) SetLocale(_ context.Context, _ Tx, _ string, locale string) error {
	r.calls++
	r.locale = locale
	return nil
}
