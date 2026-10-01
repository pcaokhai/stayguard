package permissions

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// RoleBased gives EDIT on every building and leaves the decision to the role rules in access.Authorizer.
// It is the FAST MODE rule (docs/14 P5: staff act in every building) and is wired everywhere; Derived stays for
// SG-501, which brings per-building levels. Tenant scope is unaffected.
type RoleBased struct{}

var _ app.BuildingLevels = RoleBased{}

func (RoleBased) Levels(_ context.Context, _ app.Caller, buildingIDs []string) (map[string]access.Level, error) {
	out := make(map[string]access.Level, len(buildingIDs))
	for _, id := range buildingIDs {
		out[id] = access.EDIT
	}
	return out, nil
}
