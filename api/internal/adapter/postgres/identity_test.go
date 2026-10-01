//go:build integration

package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

var idNow = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

// idEnv is a unit of work as the app role plus the repo under test.
func idEnv(t *testing.T) (*UnitOfWork, IdentityRepo, string) {
	t.Helper()
	db := migratedDB(t)
	return NewUnitOfWork(newAppPool(t, db, 4)), IdentityRepo{}, db
}

// makeTenant creates a trial tenant, an OWNER user and a session through the real unit of work.
func makeTenant(ctx context.Context, t *testing.T, uow *UnitOfWork, r IdentityRepo, tenant string, exp time.Time) app.User {
	t.Helper()
	var u app.User
	err := uow.Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error {
		if err := r.CreateTrialTenant(ctx, tx, tenant, "Trial", exp); err != nil {
			return err
		}
		var err error
		if u, err = r.CreateUser(ctx, tx, "usr_"+tenant, "owner", access.RoleOwner, "vi"); err != nil {
			return err
		}
		return r.InsertSession(ctx, tx, "hash_"+tenant, u.ID, exp)
	})
	if err != nil {
		t.Fatalf("make tenant %s: %v", tenant, err)
	}
	return u
}

func TestIdentityRepo_SG102_AC3(t *testing.T) {
	ctx := context.Background()
	uow, r, db := idEnv(t)
	ua := makeTenant(ctx, t, uow, r, "tnt_id_a", idNow.Add(time.Hour))
	makeTenant(ctx, t, uow, r, "tnt_id_b", idNow.Add(-time.Hour))
	if _, err := connAs(t, db, "owner").Exec(ctx, `INSERT INTO app.tenants (id, name) VALUES ('tnt_id_plain', 'p')`); err != nil {
		t.Fatal(err)
	}

	t.Run("trial flag", func(t *testing.T) {
		cases := map[string]bool{"tnt_id_a": true, "tnt_id_b": false, "tnt_id_plain": false}
		for id, want := range cases {
			_ = uow.Do(ctx, id, func(ctx context.Context, tx app.Tx) error {
				got, err := r.TrialTenant(ctx, tx, id, idNow)
				if err != nil || got != want {
					t.Errorf("TrialTenant(%s) = %v err=%v, want %v", id, got, err, want)
				}
				return nil
			})
		}
	})
	t.Run("reads and locale", func(t *testing.T) {
		err := uow.Do(ctx, "tnt_id_a", func(ctx context.Context, tx app.Tx) error {
			if got, ok, err := r.UserByRole(ctx, tx, access.RoleOwner); err != nil || !ok || got.ID != ua.ID {
				t.Errorf("UserByRole = %+v %v %v", got, ok, err)
			}
			if _, ok, _ := r.UserByRole(ctx, tx, access.RoleHousekeeping); ok {
				t.Error("no housekeeping user expected")
			}
			if err := r.SetLocale(ctx, tx, ua.ID, "en"); err != nil {
				return err
			}
			info, err := r.TenantInfo(ctx, tx)
			if err != nil || info.ID != "tnt_id_a" || info.Currency != "VND" {
				t.Errorf("TenantInfo = %+v err=%v", info, err)
			}
			ids, err := r.BuildingIDs(ctx, tx)
			if err != nil || len(ids) != 0 {
				t.Errorf("BuildingIDs = %v err=%v", ids, err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = uow.Do(ctx, "tnt_id_a", func(ctx context.Context, tx app.Tx) error {
			if u, ok, err := r.UserByID(ctx, tx, ua.ID); err != nil || !ok || u.Locale != "en" {
				t.Errorf("locale not persisted: %+v %v %v", u, ok, err)
			}
			return nil
		})
	})
	t.Run("cross tenant", func(t *testing.T) {
		_ = uow.Do(ctx, "tnt_id_b", func(ctx context.Context, tx app.Tx) error {
			if _, ok, err := r.UserByID(ctx, tx, ua.ID); err != nil || ok {
				t.Errorf("B read A's user: ok=%v err=%v", ok, err)
			}
			if _, err := r.TenantInfo(ctx, tx); err != nil {
				t.Errorf("B own info: %v", err)
			}
			if got, err := r.TrialTenant(ctx, tx, "tnt_id_a", idNow); got || !errors.Is(err, ErrTenantMismatch) {
				t.Errorf("B saw A's tenant: got=%v err=%v", got, err)
			}
			err := r.CreateTrialTenant(ctx, tx, "tnt_id_a", "x", idNow)
			if !errors.Is(err, ErrTenantMismatch) || strings.Contains(err.Error(), "tnt_id") {
				t.Errorf("CreateTrialTenant with A's id: %v", err)
			}
			if err := r.SetLocale(ctx, tx, ua.ID, "vi"); err == nil {
				t.Error("B changed A's locale")
			}
			if err := r.InsertSession(ctx, tx, "hash_x", ua.ID, idNow); err == nil {
				t.Error("B created a session for A's user")
			}
			return errors.New("rollback")
		})
	})
}

func TestUniqueViolation_SG102_AC1(t *testing.T) {
	ctx := context.Background()
	uow, r, _ := idEnv(t)
	makeTenant(ctx, t, uow, r, "tnt_uv", idNow.Add(time.Hour))
	err := uow.Do(ctx, "tnt_uv", func(ctx context.Context, tx app.Tx) error {
		_, err := r.CreateUser(ctx, tx, "usr_dup", "owner", access.RoleOwner, "vi")
		return err
	})
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("err = %v, want app.ErrConflict", err)
	}
	for _, leak := range []string{"owner", "usr_dup", "tnt_uv", "users_", "constraint", "Key ("} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q leaks %q", err, leak)
		}
	}
}

func TestCreateUserConflictKeepsTx_SG102_AC1(t *testing.T) {
	ctx := context.Background()
	uow, r, _ := idEnv(t)
	makeTenant(ctx, t, uow, r, "tnt_kt", idNow.Add(time.Hour))
	err := uow.Do(ctx, "tnt_kt", func(ctx context.Context, tx app.Tx) error {
		if _, err := r.CreateUser(ctx, tx, "usr_dup", "owner", access.RoleOwner, "vi"); !errors.Is(err, app.ErrConflict) {
			t.Fatalf("want conflict, got %v", err)
		}
		if _, ok, err := r.UserByRole(ctx, tx, access.RoleOwner); err != nil || !ok {
			t.Errorf("transaction unusable after conflict: ok=%v err=%v", ok, err)
		}
		_, err := r.CreateUser(ctx, tx, "usr_two", "second", access.RoleReceptionist, "vi")
		return err
	})
	if err != nil {
		t.Fatalf("unit of work failed after savepoint rollback: %v", err)
	}
}

func TestSessionsTable_SG102_AC2(t *testing.T) {
	ctx := context.Background()
	uow, r, db := idEnv(t)
	token := "raw-token-value-not-to-be-stored"
	hash := app.HashToken(token)
	err := uow.Do(ctx, "tnt_st", func(ctx context.Context, tx app.Tx) error {
		if err := r.CreateTrialTenant(ctx, tx, "tnt_st", "Trial", idNow.Add(time.Hour)); err != nil {
			return err
		}
		u, err := r.CreateUser(ctx, tx, "usr_st", "owner", access.RoleOwner, "vi")
		if err != nil {
			return err
		}
		return r.InsertSession(ctx, tx, hash, u.ID, idNow.Add(time.Hour))
	})
	if err != nil {
		t.Fatal(err)
	}
	var stored, row string
	owner := connAs(t, db, "owner")
	if err := owner.QueryRow(ctx, `SELECT token_hash, s::text FROM app.sessions s WHERE tenant_id = 'tnt_st'`).Scan(&stored, &row); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(stored) || stored != hash {
		t.Errorf("token_hash %q is not the SHA-256 hex", stored)
	}
	if strings.Contains(row, token) {
		t.Error("a column holds the raw token")
	}
}
