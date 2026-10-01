// Package permissions resolves a caller's building access.
package permissions

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// Derived implements app.BuildingLevels without stored permissions: the owner has EDIT on every
// building and everyone else NONE (the SG-102 derivation). SG-501 replaces it with stored levels.
type Derived struct{}

var _ app.BuildingLevels = Derived{}

func (Derived) Levels(_ context.Context, c app.Caller, buildingIDs []string) (map[string]access.Level, error) {
	level := access.EffectiveLevel(c.Role, access.NONE)
	out := make(map[string]access.Level, len(buildingIDs))
	for _, id := range buildingIDs {
		out[id] = level
	}
	return out, nil
}
