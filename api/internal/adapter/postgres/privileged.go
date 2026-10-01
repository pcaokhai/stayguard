package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPrivilegedRole is a fixed message on purpose: it never names the role or the URL.
var ErrPrivilegedRole = errors.New("database role is privileged (superuser, BYPASSRLS, member of such a role, or owner of app tables): connect as the application role")

// privilegedSQL is true when the connected role could bypass RLS or change the schema: RLS is the second guard (ADR-005).
const privilegedSQL = `SELECT r.rolsuper OR r.rolbypassrls
	OR EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relowner = r.oid)
	OR EXISTS (SELECT 1 FROM pg_namespace n WHERE n.nspname = 'app' AND n.nspowner = r.oid)
	OR EXISTS (SELECT 1 FROM pg_roles m WHERE m.oid <> r.oid AND (m.rolsuper OR m.rolbypassrls)
		AND pg_has_role(r.oid, m.oid, 'MEMBER'))
	FROM pg_roles r WHERE r.rolname = current_user`

// checkUnprivileged fails when the pool's role is privileged. Stricter than the migration guards, which
// only look at the roles SG-003 creates.
func checkUnprivileged(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	var privileged bool
	if err := pool.QueryRow(ctx, privilegedSQL).Scan(&privileged); err != nil {
		return fmt.Errorf("check database role: %w", err)
	}
	if privileged {
		return ErrPrivilegedRole
	}
	return nil
}
