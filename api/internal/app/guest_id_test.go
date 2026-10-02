package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

type fakeGuestRepo struct {
	stay   GuestIDStay
	row    *GuestIDRow
	photos map[string][]byte
	days   int
	purged []string
}

func (r *fakeGuestRepo) Stay(context.Context, Tx, string) (GuestIDStay, bool, error) {
	return r.stay, true, nil
}
func (r *fakeGuestRepo) SetNumber(_ context.Context, _ Tx, _ string, enc []byte, at time.Time, _ string) error {
	if r.row == nil {
		r.row = &GuestIDRow{CollectedAt: at}
	}
	r.row.NumberEnc = enc
	return nil
}
func (r *fakeGuestRepo) EnsureRecord(_ context.Context, _ Tx, _ string, at time.Time, _ string) error {
	if r.row == nil {
		r.row = &GuestIDRow{CollectedAt: at}
	}
	return nil
}
func (r *fakeGuestRepo) Record(context.Context, Tx, string) (GuestIDRow, bool, error) {
	if r.row == nil {
		return GuestIDRow{}, false, nil
	}
	out := *r.row
	out.Photos = nil
	for side, b := range r.photos {
		out.Photos = append(out.Photos, GuestPhotoMeta{Side: side, Bytes: len(b)})
	}
	return out, true, nil
}
func (r *fakeGuestRepo) ClearNumber(context.Context, Tx, string) (bool, error) {
	if r.row == nil || r.row.NumberEnc == nil {
		return false, nil
	}
	r.row.NumberEnc = nil
	return true, nil
}
func (r *fakeGuestRepo) PutPhoto(_ context.Context, _ Tx, _, side string, enc []byte, _ int, _ time.Time, _ string) error {
	if r.photos == nil {
		r.photos = map[string][]byte{}
	}
	r.photos[side] = enc
	return nil
}
func (r *fakeGuestRepo) Photo(_ context.Context, _ Tx, _, side string) ([]byte, bool, error) {
	b, ok := r.photos[side]
	return b, ok, nil
}
func (r *fakeGuestRepo) DeletePhoto(_ context.Context, _ Tx, _, side string) (bool, error) {
	_, ok := r.photos[side]
	delete(r.photos, side)
	return ok, nil
}
func (r *fakeGuestRepo) RetentionDays(context.Context, Tx) (int, error) { return r.days, nil }
func (r *fakeGuestRepo) Expired(_ context.Context, _ Tx, cutoff time.Time) ([]string, error) {
	if r.stay.CheckOutAt != nil && r.stay.CheckOutAt.Before(cutoff) {
		return []string{r.stay.ID}, nil
	}
	return nil, nil
}
func (r *fakeGuestRepo) DeleteAll(_ context.Context, _ Tx, id string) error {
	r.purged = append(r.purged, id)
	r.row, r.photos = nil, nil
	return nil
}

type fakeSanitizer struct{ err error }

func (f fakeSanitizer) Sanitize(raw []byte) ([]byte, error) {
	return append([]byte("clean:"), raw...), f.err
}

type guestRig struct {
	g     *GuestIDs
	repo  *fakeGuestRepo
	audit *fakeAudit
	clock *clockBox
}

func newGuestRig(levels map[string]access.Level) guestRig {
	repo := &fakeGuestRepo{stay: GuestIDStay{ID: "st1", Status: "ACTIVE", BuildingID: "b1", RoomCode: "A1"}, days: 30}
	audit, clk := &fakeAudit{}, &clockBox{now: t0}
	g := NewGuestIDs(&fakeUoW{}, repo, &fakeLevels{levels: levels}, fakeEnc{}, fakeSanitizer{}, audit, &seqIDs{}, clk)
	return guestRig{g: g, repo: repo, audit: audit, clock: clk}
}

func callerOf(role access.Role) Caller {
	return Caller{TenantID: "tn_a", UserID: "us_" + string(role), Role: role}
}

// SG-805 AC3: one table over every operation and role.
func TestGuestIDOperationsByRole_SG805_AC3(t *testing.T) {
	r := newGuestRig(map[string]access.Level{"b1": access.EDIT})
	ctx := context.Background()
	if _, err := r.g.SetNumber(ctx, callerOf(access.RoleReceptionist), "st1", "079203001234"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.g.UploadPhoto(ctx, callerOf(access.RoleReceptionist), "st1", "FRONT", []byte("img")); err != nil {
		t.Fatal(err)
	}
	reads := map[string]func(Caller) error{
		"record": func(c Caller) error { _, err := r.g.Record(ctx, c, "st1"); return err },
		"reveal": func(c Caller) error { _, err := r.g.Reveal(ctx, c, "st1"); return err },
		"photo":  func(c Caller) error { _, err := r.g.Photo(ctx, c, "st1", "FRONT", false); return err },
	}
	writes := map[string]func(Caller) error{
		"set": func(c Caller) error { _, err := r.g.SetNumber(ctx, c, "st1", "079203001234"); return err },
		"upload": func(c Caller) error {
			_, err := r.g.UploadPhoto(ctx, c, "st1", "BACK", []byte("img"))
			return err
		},
	}
	for role, readsOK := range map[access.Role]bool{access.RoleOwner: true, access.RoleManager: true, access.RoleReceptionist: false, access.RoleHousekeeping: false} {
		for name, op := range reads {
			if err := op(callerOf(role)); readsOK != (err == nil) || (!readsOK && !errors.Is(err, access.ErrRoleForbidden)) {
				t.Errorf("%s %s: %v", role, name, err)
			}
		}
	}
	for role, writesOK := range map[access.Role]bool{access.RoleOwner: true, access.RoleManager: true, access.RoleReceptionist: true, access.RoleHousekeeping: false} {
		for name, op := range writes {
			err := op(Caller{TenantID: "tn_a", UserID: "u", Role: role, Levels: map[string]access.Level{"b1": access.EDIT}})
			if writesOK != (err == nil) || (!writesOK && !errors.Is(err, access.ErrRoleForbidden)) {
				t.Errorf("%s %s: %v", role, name, err)
			}
		}
	}
}

func TestGuestID_MaskedRecordNeverHoldsTheNumber_SG805(t *testing.T) {
	r := newGuestRig(map[string]access.Level{"b1": access.EDIT})
	ctx := context.Background()
	if _, err := r.g.SetNumber(ctx, callerOf(access.RoleOwner), "st1", "079203001234"); err != nil {
		t.Fatal(err)
	}
	r.g.levels.(*fakeLevels).levels["b1"] = access.VIEW // a manager who can only view the building
	v, err := r.g.Record(ctx, callerOf(access.RoleManager), "st1")
	if err != nil || v.IDNumberMasked == nil || *v.IDNumberMasked != "079******234" || !v.Indicators.HasIDNumber {
		t.Fatalf("%+v %v", v, err)
	}
	for _, s := range []string{"079203001234", "203001"} {
		if strings.Contains(*v.IDNumberMasked, s) {
			t.Fatal("masked number leaks")
		}
	}
	for _, e := range r.audit.entries {
		if strings.Contains(string(e.After), "0792") {
			t.Fatalf("number in audit: %s", e.After)
		}
	}
	if MaskGuestID("123456") != "******" || MaskGuestID("123456789") != "123***789" {
		t.Fatal("mask")
	}
}

// A ciphertext copied to another stay (or from the number field to a photo) does not open.
func TestGuestID_CiphertextBoundToStayAndField_SG805(t *testing.T) {
	r := newGuestRig(map[string]access.Level{"b1": access.EDIT})
	ctx := context.Background()
	owner := callerOf(access.RoleOwner)
	_, _ = r.g.SetNumber(ctx, owner, "st1", "079203001234")
	r.repo.stay.ID = "st2"
	if _, err := r.g.Reveal(ctx, owner, "st2"); err == nil || strings.Contains(err.Error(), "079203001234") {
		t.Fatalf("a number moved to another stay must fail to open: %v", err)
	}
}

func TestGuestID_NoConsentAsked_WindowApplies_SG805_AC1(t *testing.T) {
	r := newGuestRig(map[string]access.Level{"b1": access.EDIT})
	ctx := context.Background()
	desk := callerOf(access.RoleReceptionist)
	desk.Levels = map[string]access.Level{"b1": access.EDIT}
	// The ID is collected under the stay-declaration duty: a photo or number needs no consent and starts the record.
	if _, err := r.g.UploadPhoto(ctx, desk, "st1", "FRONT", []byte("x")); err != nil || r.repo.row == nil || string(r.repo.photos["FRONT"]) == "clean:x" {
		t.Fatalf("photo without any consent, stored encrypted: %v", err)
	}
	if _, err := r.g.UploadPhoto(ctx, desk, "st1", "BACK", []byte("x")); err != nil {
		t.Fatalf("second photo: %v", err)
	}
	v, err := r.g.Record(ctx, callerOf(access.RoleOwner), "st1")
	if err != nil || v.LegalBasis != "STAY_DECLARATION" || v.CollectedAt == nil {
		t.Fatalf("record carries its legal basis: %+v %v", v, err)
	}
	// Checked out: open for 24 hours, closed after.
	out := t0.Add(-23 * time.Hour)
	r.repo.stay.Status, r.repo.stay.CheckOutAt = "CHECKED_OUT", &out
	if _, err := r.g.SetNumber(ctx, desk, "st1", "079203001234"); err != nil {
		t.Fatalf("23 hours after check-out: %v", err)
	}
	out = t0.Add(-25 * time.Hour)
	if _, err := r.g.SetNumber(ctx, desk, "st1", "079203001234"); !errors.Is(err, ErrGuestIDClosed) {
		t.Fatalf("25 hours after check-out: %v", err)
	}
	r.g.sanitizer = fakeSanitizer{err: ErrPhotoInvalid}
	r.repo.stay.Status, r.repo.stay.CheckOutAt = "ACTIVE", nil
	var ve *ValidationError
	if _, err := r.g.UploadPhoto(ctx, desk, "st1", "FRONT", []byte("x")); !errors.As(err, &ve) {
		t.Fatalf("undecodable image: %v", err)
	}
	r.g.sanitizer = fakeSanitizer{err: ErrPhotoTooLarge}
	if _, err := r.g.UploadPhoto(ctx, desk, "st1", "FRONT", []byte("x")); !errors.Is(err, ErrPhotoTooLarge) {
		t.Fatalf("too large: %v", err)
	}
}

func TestGuestID_PurgeUsesRetentionDays_SG805_AC5(t *testing.T) {
	r := newGuestRig(map[string]access.Level{"b1": access.EDIT})
	out := t0.Add(-31 * 24 * time.Hour)
	r.repo.stay.Status, r.repo.stay.CheckOutAt = "CHECKED_OUT", &out
	n, err := r.g.Purge(context.Background(), "tn_a", t0)
	if err != nil || n != 1 || len(r.repo.purged) != 1 || len(r.audit.entries) != 1 || r.audit.entries[0].Action != "GUEST_ID_RETENTION_DELETED" || r.audit.entries[0].ActorID != "" {
		t.Fatalf("%d %v %+v", n, err, r.audit.entries)
	}
	r.repo.days = 60
	if n, _ = r.g.Purge(context.Background(), "tn_a", t0); n != 0 {
		t.Fatal("within retention nothing goes")
	}
}
