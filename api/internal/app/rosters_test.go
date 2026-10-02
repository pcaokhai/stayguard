package app

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/roster"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

type fakeRosterRepo struct {
	cells  map[RosterCell]bool
	leave  map[string]*LeaveRow
	people map[string]bool
	annual int
}

func (r *fakeRosterRepo) Timezone(context.Context, Tx) (string, error) { return zoneName, nil }
func (r *fakeRosterRepo) Assignments(_ context.Context, _ Tx, from, to time.Time) ([]RosterCell, error) {
	var out []RosterCell
	for c := range r.cells {
		if !c.Date.Before(from) && !c.Date.After(to) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Date.Before(out[j].Date) || (out[i].Date.Equal(out[j].Date) && out[i].UserID < out[j].UserID)
	})
	return out, nil
}
func (r *fakeRosterRepo) ActiveUsers(_ context.Context, _ Tx, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = r.people[id]
	}
	return out, nil
}
func (r *fakeRosterRepo) Remove(_ context.Context, _ Tx, cs []RosterCell) error {
	for _, c := range cs {
		delete(r.cells, c)
	}
	return nil
}
func (r *fakeRosterRepo) Add(_ context.Context, _ Tx, cs []RosterCell, _ string) error {
	for _, c := range cs {
		r.cells[c] = true
	}
	return nil
}
func (r *fakeRosterRepo) CountBetween(ctx context.Context, tx Tx, from, to time.Time) (int, error) {
	cs, _ := r.Assignments(ctx, tx, from, to)
	return len(cs), nil
}
func (r *fakeRosterRepo) CopyWeek(_ context.Context, _ Tx, src time.Time, _ string) error {
	for c := range r.cells {
		if !c.Date.Before(src) && c.Date.Before(src.AddDate(0, 0, 7)) {
			r.cells[RosterCell{UserID: c.UserID, Date: c.Date.AddDate(0, 0, 7), Shift: c.Shift}] = true
		}
	}
	return nil
}
func (r *fakeRosterRepo) Scheduled(context.Context, Tx, string, time.Time) ([]string, error) {
	return nil, nil
}
func (r *fakeRosterRepo) Leave(_ context.Context, _ Tx, f LeaveFilter) ([]LeaveRow, error) {
	var out []LeaveRow
	for _, l := range r.leave {
		touches := f.From == nil || (!l.To.Before(*f.From) && !l.From.After(*f.To))
		standing := !f.Standing || l.Status == "PENDING" || l.Status == "APPROVED" || l.Status == "CANCEL_REQUESTED"
		if touches && standing && (f.UserID == "" || l.UserID == f.UserID) && (f.Status == "" || l.Status == f.Status) {
			out = append(out, *l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (r *fakeRosterRepo) LeaveByID(_ context.Context, _ Tx, id string) (LeaveRow, bool, error) {
	l, ok := r.leave[id]
	if !ok {
		return LeaveRow{}, false, nil
	}
	return *l, true, nil
}
func (r *fakeRosterRepo) LockLeave(_ context.Context, _ Tx, id string) (bool, error) {
	_, ok := r.leave[id]
	return ok, nil
}
func (r *fakeRosterRepo) InsertLeave(_ context.Context, _ Tx, l LeaveRow) error {
	l.UserName = "Person " + l.UserID
	r.leave[l.ID] = &l
	return nil
}
func (r *fakeRosterRepo) SetLeaveStatus(_ context.Context, _ Tx, id, status, decline string, _ time.Time, _ string) error {
	r.leave[id].Status, r.leave[id].DeclineReason = status, decline
	return nil
}
func (r *fakeRosterRepo) OverlappingLeave(_ context.Context, _ Tx, user string, from, to time.Time) (int, error) {
	n := 0
	for _, l := range r.leave {
		if l.UserID == user && (l.Status == "PENDING" || l.Status == "APPROVED" || l.Status == "CANCEL_REQUESTED") && !l.To.Before(from) && !l.From.After(to) {
			n++
		}
	}
	return n, nil
}
func (r *fakeRosterRepo) PaidLeaveDays(_ context.Context, _ Tx, user string, _, _ time.Time) (int, error) {
	days := 0
	for _, l := range r.leave {
		if l.UserID == user && l.Kind == "PAID" && (l.Status == "APPROVED" || l.Status == "CANCEL_REQUESTED") {
			days += roster.Days(l.From, l.To)
		}
	}
	return days, nil
}
func (r *fakeRosterRepo) AnnualLeaveDays(context.Context, Tx, string) (int, error) {
	return r.annual, nil
}

type rosterEnv struct {
	r      *Rosters
	repo   *fakeRosterRepo
	alerts *fakeAlerts
	owner  Caller
	mgr    Caller
	lan    Caller
	clock  *rosterClock
}

type rosterClock struct{ t time.Time }

func (c *rosterClock) Now() time.Time { return c.t }

func jan(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) } // 2 January 2026 is a Friday, 5 a Monday

func newRosterEnv() *rosterEnv {
	repo := &fakeRosterRepo{cells: map[RosterCell]bool{}, leave: map[string]*LeaveRow{}, annual: 12,
		people: map[string]bool{"u_lan": true, "u_ba": true, "u_chi": true}}
	e := &rosterEnv{repo: repo, alerts: &fakeAlerts{}, clock: &rosterClock{t0},
		owner: Caller{TenantID: tenantA, UserID: "u_o", Role: access.RoleOwner},
		mgr:   Caller{TenantID: tenantA, UserID: "u_m", Role: access.RoleManager},
		lan:   Caller{TenantID: tenantA, UserID: "u_lan", Role: access.RoleReceptionist}}
	idem := &fakeIdem{m: map[string]idemState{}}
	e.r = NewRosters(&rollbackUoW{idem: idem}, repo, &fakeLevels{levels: map[string]access.Level{}}, idem, &fakeAudit{}, e.alerts, &seqIDs{}, e.clock)
	return e
}

func (e *rosterEnv) putSet(t *testing.T, key string, cells ...RosterCell) RosterView {
	t.Helper()
	v, err := e.r.PutRoster(context.Background(), e.mgr, key, cells, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func hasGap(v RosterView, d time.Time, shift string) bool {
	for _, g := range v.Gaps {
		if g.Date.Equal(d) && g.Shift == shift {
			return true
		}
	}
	return false
}

func TestPutRoster_AssignRemoveAndGuards_SG1102(t *testing.T) {
	e := newRosterEnv()
	ctx := context.Background()
	v := e.putSet(t, "k1", RosterCell{UserID: "u_lan", Date: jan(5), Shift: "MORNING"}, RosterCell{UserID: "u_ba", Date: jan(5), Shift: "AFTERNOON"})
	if len(v.Assignments) != 2 || hasGap(v, jan(5), "MORNING") || !hasGap(v, jan(5), "NIGHT") {
		t.Fatalf("roster %+v gaps %v", v.Assignments, v.Gaps)
	}
	if again := e.putSet(t, "k1", RosterCell{UserID: "u_lan", Date: jan(5), Shift: "MORNING"}, RosterCell{UserID: "u_ba", Date: jan(5), Shift: "AFTERNOON"}); len(again.Assignments) != 2 {
		t.Fatalf("replay: %+v", again)
	}
	if v, err := e.r.PutRoster(ctx, e.mgr, "k2", nil, []RosterCell{{UserID: "u_ba", Date: jan(5), Shift: "AFTERNOON"}}); err != nil || len(v.Assignments) != 1 {
		t.Fatalf("remove: %+v %v", v, err)
	}
	var ve *stay.ValidationError
	if _, err := e.r.PutRoster(ctx, e.mgr, "k3", []RosterCell{{UserID: "u_gone", Date: jan(5), Shift: "NIGHT"}}, nil); !errors.As(err, &ve) {
		t.Errorf("unknown person: %v", err)
	}
	if _, err := e.r.PutRoster(ctx, e.mgr, "k4", []RosterCell{{UserID: "u_lan", Date: jan(5), Shift: "EVENING"}}, nil); !errors.As(err, &ve) {
		t.Errorf("bad shift: %v", err)
	}
	if _, err := e.r.PutRoster(ctx, e.lan, "k5", []RosterCell{{UserID: "u_lan", Date: jan(5), Shift: "NIGHT"}}, nil); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("receptionist edits the roster: %v", err)
	}
}

func TestRoster_CoverAndGaps_SG1102(t *testing.T) {
	e := newRosterEnv()
	ctx := context.Background()
	e.putSet(t, "k1", RosterCell{UserID: "u_lan", Date: jan(6), Shift: "MORNING"}, RosterCell{UserID: "u_lan", Date: jan(6), Shift: "AFTERNOON"}, RosterCell{UserID: "u_ba", Date: jan(6), Shift: "NIGHT"})
	// lan takes the 6th off, covered by chi; the cover only fills lan's own shifts.
	l, err := e.r.CreateLeave(ctx, e.lan, "kl", CreateLeaveInput{From: jan(6), To: jan(6), Kind: "PAID", CoverID: "u_chi"})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := e.r.GetRoster(ctx, e.owner, jan(6), jan(6)); !hasGap(v, jan(6), "MORNING") == false || len(v.Leave) != 1 {
		t.Fatalf("pending leave must not open a gap: %+v", v.Gaps)
	}
	if _, err := e.r.Approve(ctx, e.owner, "ka", l.ID); err != nil {
		t.Fatal(err)
	}
	v, err := e.r.GetRoster(ctx, e.owner, jan(6), jan(6))
	if err != nil || hasGap(v, jan(6), "MORNING") || hasGap(v, jan(6), "AFTERNOON") || hasGap(v, jan(6), "NIGHT") {
		t.Fatalf("covered leave left gaps: %v %v", v.Gaps, err)
	}
	// nobody to cover: the same leave without cover leaves lan's shifts uncovered
	e.repo.leave[l.ID].CoverUserID = ""
	if v, _ = e.r.GetRoster(ctx, e.owner, jan(6), jan(6)); !hasGap(v, jan(6), "MORNING") || !hasGap(v, jan(6), "AFTERNOON") || hasGap(v, jan(6), "NIGHT") {
		t.Fatalf("uncovered leave: %v", v.Gaps)
	}
	// and nobody can be rostered onto a day they are off
	if _, err := e.r.PutRoster(ctx, e.mgr, "k9", []RosterCell{{UserID: "u_lan", Date: jan(6), Shift: "NIGHT"}}, nil); !errors.Is(err, ErrLeaveConflict) {
		t.Fatalf("rostering a person on leave: %v", err)
	}
	if mine, err := e.r.MyRoster(ctx, e.lan, jan(6), jan(6)); err != nil || len(mine.Assignments) != 2 || mine.Gaps != nil {
		t.Fatalf("my roster: %+v %v", mine, err)
	}
}

func TestCopyWeek_SG1102(t *testing.T) {
	e := newRosterEnv()
	ctx := context.Background()
	e.putSet(t, "k1", RosterCell{UserID: "u_lan", Date: jan(5), Shift: "MORNING"}, RosterCell{UserID: "u_ba", Date: jan(7), Shift: "NIGHT"})
	var ve *stay.ValidationError
	if _, err := e.r.CopyWeek(ctx, e.owner, "c1", jan(12)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if v, _ := e.r.GetRoster(ctx, e.owner, jan(12), jan(18)); len(v.Assignments) != 2 {
		t.Fatalf("copied week: %+v", v.Assignments)
	}
	if _, err := e.r.CopyWeek(ctx, e.owner, "c2", jan(12)); !errors.Is(err, ErrRosterNotEmpty) {
		t.Errorf("copy into a filled week: %v", err)
	}
	if _, err := e.r.CopyWeek(ctx, e.owner, "c3", jan(14)); !errors.As(err, &ve) {
		t.Errorf("not a monday: %v", err)
	}
}

func TestLeave_RequestDecideAndCancel_SG1103(t *testing.T) {
	e := newRosterEnv()
	ctx := context.Background()
	var ve *stay.ValidationError
	if _, err := e.r.CreateLeave(ctx, e.lan, "k0", CreateLeaveInput{From: jan(1), To: jan(1), Kind: "PAID"}); !errors.As(err, &ve) {
		t.Fatalf("leave in the past: %v", err)
	}
	l, err := e.r.CreateLeave(ctx, e.lan, "k1", CreateLeaveInput{From: jan(8), To: jan(10), Kind: "PAID", Reason: "family"})
	if err != nil || l.Status != "PENDING" || l.UserName == "" || len(e.alerts.raised) != 1 || e.alerts.raised[0].Kind != AlertLeaveRequested {
		t.Fatalf("create: %+v %v alerts=%v", l, err, e.alerts.raised)
	}
	if _, err := e.r.CreateLeave(ctx, e.lan, "k2", CreateLeaveInput{From: jan(10), To: jan(12), Kind: "SICK"}); !errors.Is(err, ErrLeaveConflict) {
		t.Fatalf("overlap: %v", err)
	}
	if _, err := e.r.CancelMine(ctx, Caller{TenantID: tenantA, UserID: "u_ba", Role: access.RoleReceptionist}, "kx", l.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("somebody else cancels: %v", err)
	}
	mine, err := e.r.MyLeave(ctx, e.lan)
	if err != nil || len(mine.Items) != 1 || mine.Balance.Annual != 12 || mine.Balance.Used != 0 || mine.Balance.Left != 12 || mine.Balance.Year != 2026 {
		t.Fatalf("my leave: %+v %v", mine, err)
	}
	if _, err := e.r.Decline(ctx, e.owner, "kd0", l.ID, " "); !errors.As(err, &ve) {
		t.Fatalf("decline without a reason: %v", err)
	}
	if _, err := e.r.Approve(ctx, e.lan, "kz", l.ID); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("receptionist approves: %v", err)
	}
	if v, err := e.r.Approve(ctx, e.owner, "k3", l.ID); err != nil || v.Status != "APPROVED" || v.DecidedAt == nil {
		t.Fatalf("approve: %+v %v", v, err)
	}
	if _, err := e.r.Approve(ctx, e.owner, "k4", l.ID); !errors.Is(err, ErrLeaveState) {
		t.Fatalf("approve twice: %v", err)
	}
	if m, _ := e.r.MyLeave(ctx, e.lan); m.Balance.Used != 3 || m.Balance.Left != 9 {
		t.Fatalf("balance after approval: %+v", m.Balance)
	}
	// approved leave needs the owner to let it go; declining the cancel keeps it
	if v, err := e.r.CancelMine(ctx, e.lan, "k5", l.ID); err != nil || v.Status != "CANCEL_REQUESTED" || len(e.alerts.raised) != 2 {
		t.Fatalf("cancel approved: %+v %v alerts=%d", v, err, len(e.alerts.raised))
	}
	if v, err := e.r.Decline(ctx, e.owner, "k6", l.ID, "we are short"); err != nil || v.Status != "APPROVED" {
		t.Fatalf("decline the cancel: %+v %v", v, err)
	}
	if _, err := e.r.CancelMine(ctx, e.lan, "k7", l.ID); err != nil {
		t.Fatal(err)
	}
	if v, err := e.r.Approve(ctx, e.owner, "k8", l.ID); err != nil || v.Status != "CANCELLED" {
		t.Fatalf("approve the cancel: %+v %v", v, err)
	}
	if _, err := e.r.CancelMine(ctx, e.lan, "k9", l.ID); !errors.Is(err, ErrLeaveState) {
		t.Fatalf("cancel a cancelled request: %v", err)
	}
}

func TestLeave_PendingCancelDeclineAndTaken_SG1103(t *testing.T) {
	e := newRosterEnv()
	ctx := context.Background()
	p, _ := e.r.CreateLeave(ctx, e.lan, "k1", CreateLeaveInput{From: jan(8), To: jan(8), Kind: "UNPAID"})
	if v, err := e.r.CancelMine(ctx, e.lan, "k2", p.ID); err != nil || v.Status != "CANCELLED" {
		t.Fatalf("cancel pending: %+v %v", v, err)
	}
	d, _ := e.r.CreateLeave(ctx, e.lan, "k3", CreateLeaveInput{From: jan(9), To: jan(9), Kind: "SICK"})
	if v, err := e.r.Decline(ctx, e.owner, "k4", d.ID, "no cover"); err != nil || v.Status != "DECLINED" || v.DeclineReason != "no cover" {
		t.Fatalf("decline: %+v %v", v, err)
	}
	// a manager cannot decide their own request; once approved leave is over it reads TAKEN and can no longer be cancelled
	mine, _ := e.r.CreateLeave(ctx, e.mgr, "k5", CreateLeaveInput{From: jan(12), To: jan(12), Kind: "PAID"})
	if _, err := e.r.Approve(ctx, e.mgr, "k6", mine.ID); !errors.Is(err, access.ErrRoleForbidden) {
		t.Fatalf("manager approves own leave: %v", err)
	}
	if _, err := e.r.Approve(ctx, e.owner, "k7", mine.ID); err != nil {
		t.Fatal(err)
	}
	e.clock.t = time.Date(2026, 1, 20, 3, 0, 0, 0, time.UTC)
	if rows, _ := e.r.ListLeave(ctx, e.owner, "TAKEN"); len(rows) != 1 || rows[0].ID != mine.ID || rows[0].Status != "TAKEN" {
		t.Fatalf("taken: %+v", rows)
	}
	if _, err := e.r.CancelMine(ctx, e.mgr, "k8", mine.ID); !errors.Is(err, ErrLeaveState) {
		t.Fatalf("cancel taken leave: %v", err)
	}
	var ve *stay.ValidationError
	if _, err := e.r.ListLeave(ctx, e.owner, "WEIRD"); !errors.As(err, &ve) {
		t.Fatalf("unknown status: %v", err)
	}
}
