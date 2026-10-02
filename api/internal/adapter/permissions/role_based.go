package permissions

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// RoleBased gives EDIT on every building and leaves the decision to the role rules in access.Authorizer.
// Production uses Stored; RoleBased keeps end-to-end tests that seed users without building access simple.
type RoleBased struct{}

var _ app.BuildingLevels = RoleBased{}

func (RoleBased) Levels(_ context.Context, _ app.Caller, buildingIDs []string) (map[string]access.Level, error) {
	out := make(map[string]access.Level, len(buildingIDs))
	for _, id := range buildingIDs {
		out[id] = access.EDIT
	}
	return out, nil
}
