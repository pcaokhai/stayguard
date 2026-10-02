package permissions

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// Stored resolves building access from the levels stored per person (building_permissions), which the
// authentication middleware loads into the Caller on every request, so a change applies on the next call.
// The owner has implicit EDIT; a missing building is NONE.
type Stored struct{}

var _ app.BuildingLevels = Stored{}

func (Stored) Levels(_ context.Context, c app.Caller, buildingIDs []string) (map[string]access.Level, error) {
	out := make(map[string]access.Level, len(buildingIDs))
	for _, id := range buildingIDs {
		out[id] = access.EffectiveLevel(c.Role, c.Levels[id])
	}
	return out, nil
}
