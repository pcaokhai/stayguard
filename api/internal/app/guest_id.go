package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const (
	// idEditWindow is how long after check-out the number and photos can still be added or retaken (docs/15 rule 21).
	idEditWindow = 24 * time.Hour
	idPhotoField = "stays.id_photo"

	auditGuestIDNumberSet      = "GUEST_ID_NUMBER_SET"
	auditGuestIDPhotoSet       = "GUEST_ID_PHOTO_SET"
	auditGuestIDRevealed       = "GUEST_ID_REVEALED"
	auditGuestIDPhotoViewed    = "GUEST_ID_PHOTO_VIEWED"
	auditGuestIDPhotoDownload  = "GUEST_ID_PHOTO_DOWNLOADED"
	auditGuestIDNumberDeleted  = "GUEST_ID_NUMBER_DELETED"
	auditGuestIDPhotoDeleted   = "GUEST_ID_PHOTO_DELETED"
	auditGuestIDRetentionPurge = "GUEST_ID_RETENTION_DELETED"
)

var (
	// ErrPhotoTooLarge: the upload is over 5 MB (HTTP 413 PHOTO_TOO_LARGE).
	ErrPhotoTooLarge = errors.New("photo too large")
	// ErrUnsupportedMedia: the upload is not a JPEG or PNG (HTTP 415 UNSUPPORTED_MEDIA).
	ErrUnsupportedMedia = errors.New("unsupported media type")
	// ErrPhotoInvalid: the upload is a JPEG or PNG by its header but cannot be decoded (HTTP 422).
	ErrPhotoInvalid = errors.New("photo cannot be decoded")
	// ErrGuestIDClosed: the stay ended more than 24 hours ago, so the guest ID can no longer be added or changed.
	ErrGuestIDClosed = errors.New("guest id can no longer be changed")
)

// GuestIDStay is the stay a guest ID belongs to.
type GuestIDStay struct {
	ID, Status, BuildingID, RoomCode string
	CheckOutAt                       *time.Time
}

type GuestPhotoMeta struct {
	Side       string
	Bytes      int
	UploadedAt time.Time
	UploadedBy string
}

// GuestIDRow is what is stored for a stay: the encrypted number (nil when none), the collection record and photo metadata.
type GuestIDRow struct {
	NumberEnc   []byte
	CollectedAt time.Time
	Photos      []GuestPhotoMeta
}

// GuestIDRepo is the only door to guest ID data. Photos go in and out as ciphertext.
type GuestIDRepo interface {
	// Stay locks the stay row; false when unknown.
	Stay(ctx context.Context, tx Tx, stayID string) (GuestIDStay, bool, error)
	SetNumber(ctx context.Context, tx Tx, stayID string, enc []byte, at time.Time, by string) error
	EnsureRecord(ctx context.Context, tx Tx, stayID string, at time.Time, by string) error
	Record(ctx context.Context, tx Tx, stayID string) (GuestIDRow, bool, error)
	ClearNumber(ctx context.Context, tx Tx, stayID string) (bool, error)
	PutPhoto(ctx context.Context, tx Tx, stayID, side string, enc []byte, size int, at time.Time, by string) error
	Photo(ctx context.Context, tx Tx, stayID, side string) ([]byte, bool, error)
	DeletePhoto(ctx context.Context, tx Tx, stayID, side string) (bool, error)
	RetentionDays(ctx context.Context, tx Tx) (int, error)
	Expired(ctx context.Context, tx Tx, cutoff time.Time) ([]string, error)
	DeleteAll(ctx context.Context, tx Tx, stayID string) error
}

// ImageSanitizer turns an upload into a metadata-free JPEG, or says why it cannot (ErrPhotoTooLarge,
// ErrUnsupportedMedia or another error for an image that cannot be decoded).
type ImageSanitizer interface {
	Sanitize(raw []byte) ([]byte, error)
}

// GuestIDs is the guest ID number and photos of a stay (docs/15 rules 21 to 25). Front desk and housekeeping can
// only write; reads are for the owner and manager, each one audited.
type GuestIDs struct {
	uow       UnitOfWork
	repo      GuestIDRepo
	levels    BuildingLevels
	enc       Encryptor
	sanitizer ImageSanitizer
	audit     AuditWriter
	ids       IDGenerator
	clock     Clock
	authz     access.Authorizer
}

func NewGuestIDs(uow UnitOfWork, repo GuestIDRepo, levels BuildingLevels, enc Encryptor, sanitizer ImageSanitizer, audit AuditWriter, ids IDGenerator, clock Clock) *GuestIDs {
	return &GuestIDs{uow: uow, repo: repo, levels: levels, enc: enc, sanitizer: sanitizer, audit: audit, ids: ids, clock: clock}
}

func photoField(stayID, side string) string { return idPhotoField + "/" + stayID + "/" + side }

func validSide(side string) bool { return side == "FRONT" || side == "BACK" }

func validIDNumber(n string) bool { return len(n) >= 9 && len(n) <= 12 && digits(n, 9, 12) }

// stayFor loads the stay and applies the building rule: EDIT to write, VIEW to read (the owner has both everywhere).
func (g *GuestIDs) stayFor(ctx context.Context, tx Tx, c Caller, op, stayID string) (GuestIDStay, error) {
	st, ok, err := g.repo.Stay(ctx, tx, stayID)
	if err != nil || !ok {
		return GuestIDStay{}, errOr(err, ErrNotFound)
	}
	lv, err := g.levels.Levels(ctx, c, []string{st.BuildingID})
	if err != nil {
		return GuestIDStay{}, fmt.Errorf("building levels: %w", err)
	}
	if err = g.authz.Check(op, c.Role, lv[st.BuildingID]); err != nil {
		return GuestIDStay{}, err
	}
	return st, nil
}

// readLevel: the manager must at least see the building of the stay; the owner always does.
func (g *GuestIDs) readableStay(ctx context.Context, tx Tx, c Caller, op, stayID string) (GuestIDStay, error) {
	st, ok, err := g.repo.Stay(ctx, tx, stayID)
	if err != nil || !ok {
		return GuestIDStay{}, errOr(err, ErrNotFound)
	}
	if err = g.authz.Check(op, c.Role, access.EDIT); err != nil {
		return GuestIDStay{}, err
	}
	lv, err := g.levels.Levels(ctx, c, []string{st.BuildingID})
	if err != nil {
		return GuestIDStay{}, fmt.Errorf("building levels: %w", err)
	}
	if lv[st.BuildingID] < access.VIEW {
		return GuestIDStay{}, access.ErrBuildingForbidden
	}
	return st, nil
}

func (g *GuestIDs) editable(st GuestIDStay) error {
	if st.Status == "ACTIVE" {
		return nil
	}
	if st.CheckOutAt != nil && g.clock.Now().Before(st.CheckOutAt.Add(idEditWindow)) {
		return nil
	}
	return ErrGuestIDClosed
}

func (g *GuestIDs) record(ctx context.Context, tx Tx, c Caller, action, stayID string, after map[string]any) error {
	e := AuditEntry{ID: g.ids.New(auditPrefix), ActorID: c.UserID, Action: action, EntityType: entityStay, EntityID: stayID}
	if after != nil {
		raw, err := json.Marshal(after)
		if err != nil {
			return err
		}
		e.After = raw
	}
	return g.audit.Append(ctx, tx, e)
}

func indicatorsOf(r GuestIDRow, ok bool) GuestIDIndicators {
	var out GuestIDIndicators
	if !ok {
		return out
	}
	out.HasIDNumber = len(r.NumberEnc) > 0
	for _, p := range r.Photos {
		out.HasFrontPhoto = out.HasFrontPhoto || p.Side == "FRONT"
		out.HasBackPhoto = out.HasBackPhoto || p.Side == "BACK"
	}
	return out
}

// legalBasis says why guest ID data is kept: the duty to declare stays, not consent.
const legalBasis = "STAY_DECLARATION"

// SetNumber stores or replaces the number, encrypted. It returns indicators only.
func (g *GuestIDs) SetNumber(ctx context.Context, c Caller, stayID, idNumber string) (GuestIDIndicators, error) {
	if err := g.authz.Check("setGuestIdNumber", c.Role, access.EDIT); err != nil {
		return GuestIDIndicators{}, err
	}
	idNumber = strings.TrimSpace(idNumber)
	if !validIDNumber(idNumber) {
		return GuestIDIndicators{}, &ValidationError{Field: "idNumber", Reason: "9 to 12 digits"}
	}
	var out GuestIDIndicators
	err := g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		st, err := g.stayFor(ctx, tx, c, "setGuestIdNumber", stayID)
		if err != nil {
			return err
		}
		if err = g.editable(st); err != nil {
			return err
		}
		enc, err := g.enc.Encrypt(c.TenantID, idField(stayID), []byte(idNumber))
		if err != nil {
			return fmt.Errorf("encrypt id number: %w", err)
		}
		if err = g.repo.SetNumber(ctx, tx, stayID, enc, g.clock.Now(), c.UserID); err != nil {
			return err
		}
		if err = g.record(ctx, tx, c, auditGuestIDNumberSet, stayID, nil); err != nil {
			return err
		}
		row, ok, err := g.repo.Record(ctx, tx, stayID)
		out = indicatorsOf(row, ok)
		return err
	})
	return out, err
}

// UploadPhoto stores a sanitized, encrypted photo.
func (g *GuestIDs) UploadPhoto(ctx context.Context, c Caller, stayID, side string, raw []byte) (GuestIDIndicators, error) {
	if err := g.authz.Check("uploadGuestIdPhoto", c.Role, access.EDIT); err != nil {
		return GuestIDIndicators{}, err
	}
	if !validSide(side) {
		return GuestIDIndicators{}, &ValidationError{Field: "side", Reason: "FRONT or BACK"}
	}
	clean, err := g.sanitizer.Sanitize(raw) // before the transaction: decoding is CPU, not a reason to hold a lock
	if errors.Is(err, ErrPhotoInvalid) {
		return GuestIDIndicators{}, &ValidationError{Field: "file", Reason: "not a readable image"}
	}
	if err != nil {
		return GuestIDIndicators{}, err
	}
	var out GuestIDIndicators
	err = g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		st, err := g.stayFor(ctx, tx, c, "uploadGuestIdPhoto", stayID)
		if err != nil {
			return err
		}
		if err = g.editable(st); err != nil {
			return err
		}
		if err = g.repo.EnsureRecord(ctx, tx, stayID, g.clock.Now(), c.UserID); err != nil {
			return err
		}
		enc, err := g.enc.Encrypt(c.TenantID, photoField(stayID, side), clean)
		if err != nil {
			return fmt.Errorf("encrypt id photo: %w", err)
		}
		if err = g.repo.PutPhoto(ctx, tx, stayID, side, enc, len(clean), g.clock.Now(), c.UserID); err != nil {
			return err
		}
		if err = g.record(ctx, tx, c, auditGuestIDPhotoSet, stayID, map[string]any{"side": side}); err != nil {
			return err
		}
		row, found, err := g.repo.Record(ctx, tx, stayID)
		out = indicatorsOf(row, found)
		return err
	})
	return out, err
}

// GuestIDRecordView is the masked record the owner and manager see.
type GuestIDRecordView struct {
	Indicators     GuestIDIndicators
	IDNumberMasked *string
	LegalBasis     string
	CollectedAt    *time.Time
	DeleteAfter    *time.Time
	Photos         []GuestPhotoMeta
}

// MaskGuestID shows the first three and last three digits.
func MaskGuestID(n string) string {
	if len(n) < 7 {
		return strings.Repeat("*", len(n))
	}
	return n[:3] + strings.Repeat("*", len(n)-6) + n[len(n)-3:]
}

// Record returns the masked number and the photo metadata. It is not a reveal and writes no audit entry.
func (g *GuestIDs) Record(ctx context.Context, c Caller, stayID string) (GuestIDRecordView, error) {
	var out GuestIDRecordView
	err := g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		st, err := g.readableStay(ctx, tx, c, "getGuestIdRecord", stayID)
		if err != nil {
			return err
		}
		row, ok, err := g.repo.Record(ctx, tx, stayID)
		if err != nil {
			return err
		}
		out.Indicators = indicatorsOf(row, ok)
		if !ok {
			return nil
		}
		out.LegalBasis, out.CollectedAt, out.Photos = legalBasis, &row.CollectedAt, row.Photos
		if len(row.NumberEnc) > 0 {
			plain, err := g.enc.Decrypt(c.TenantID, idField(stayID), row.NumberEnc)
			if err != nil {
				return fmt.Errorf("guest id number: %w", err)
			}
			m := MaskGuestID(string(plain))
			out.IDNumberMasked = &m
		}
		if st.CheckOutAt != nil {
			days, err := g.repo.RetentionDays(ctx, tx)
			if err != nil {
				return err
			}
			d := st.CheckOutAt.AddDate(0, 0, days)
			out.DeleteAfter = &d
		}
		return nil
	})
	return out, err
}

// Reveal returns the full number and writes an audit entry.
func (g *GuestIDs) Reveal(ctx context.Context, c Caller, stayID string) (string, error) {
	var plain []byte
	err := g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if _, err := g.readableStay(ctx, tx, c, "revealGuestIdNumber", stayID); err != nil {
			return err
		}
		row, ok, err := g.repo.Record(ctx, tx, stayID)
		if err != nil || !ok || len(row.NumberEnc) == 0 {
			return errOr(err, ErrNotFound)
		}
		if plain, err = g.enc.Decrypt(c.TenantID, idField(stayID), row.NumberEnc); err != nil {
			return fmt.Errorf("guest id number: %w", err)
		}
		return g.record(ctx, tx, c, auditGuestIDRevealed, stayID, nil)
	})
	return string(plain), err
}

// Photo returns the decrypted JPEG for viewing or download and writes an audit entry.
func (g *GuestIDs) Photo(ctx context.Context, c Caller, stayID, side string, download bool) ([]byte, error) {
	if !validSide(side) {
		return nil, &ValidationError{Field: "side", Reason: "FRONT or BACK"}
	}
	var img []byte
	err := g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if _, err := g.readableStay(ctx, tx, c, "getGuestIdPhoto", stayID); err != nil {
			return err
		}
		enc, ok, err := g.repo.Photo(ctx, tx, stayID, side)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if img, err = g.enc.Decrypt(c.TenantID, photoField(stayID, side), enc); err != nil {
			return fmt.Errorf("guest id photo: %w", err)
		}
		action := auditGuestIDPhotoViewed
		if download {
			action = auditGuestIDPhotoDownload
		}
		return g.record(ctx, tx, c, action, stayID, map[string]any{"side": side})
	})
	return img, err
}

func (g *GuestIDs) DeletePhoto(ctx context.Context, c Caller, stayID, side string) error {
	if !validSide(side) {
		return &ValidationError{Field: "side", Reason: "FRONT or BACK"}
	}
	return g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if _, err := g.readableStay(ctx, tx, c, "deleteGuestIdPhoto", stayID); err != nil {
			return err
		}
		if ok, err := g.repo.DeletePhoto(ctx, tx, stayID, side); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		return g.record(ctx, tx, c, auditGuestIDPhotoDeleted, stayID, map[string]any{"side": side})
	})
}

func (g *GuestIDs) DeleteNumber(ctx context.Context, c Caller, stayID string) error {
	return g.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		if _, err := g.readableStay(ctx, tx, c, "deleteGuestIdNumber", stayID); err != nil {
			return err
		}
		if ok, err := g.repo.ClearNumber(ctx, tx, stayID); err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		return g.record(ctx, tx, c, auditGuestIDNumberDeleted, stayID, nil)
	})
}

// Purge deletes the numbers and photos of one tenant whose stays checked out more than idRetentionDays ago and
// records each deletion. It returns how many stays were purged. It is the per-tenant half of the daily retention job.
func (g *GuestIDs) Purge(ctx context.Context, tenantID string, now time.Time) (int, error) {
	n := 0
	err := g.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error {
		days, err := g.repo.RetentionDays(ctx, tx)
		if err != nil {
			return err
		}
		stays, err := g.repo.Expired(ctx, tx, now.AddDate(0, 0, -days))
		if err != nil {
			return err
		}
		for _, id := range stays {
			if err = g.repo.DeleteAll(ctx, tx, id); err != nil {
				return err
			}
			if err = g.record(ctx, tx, Caller{TenantID: tenantID}, auditGuestIDRetentionPurge, id, map[string]any{"retentionDays": days}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
