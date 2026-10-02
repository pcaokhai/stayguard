package permissions

import (
	"context"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

func TestStoredLevels_SG501_AC4(t *testing.T) {
	ids := []string{"b1", "b2", "b3"}
	stored := map[string]access.Level{"b1": access.EDIT, "b2": access.VIEW}
	cases := []struct {
		role access.Role
		want [3]access.Level
	}{
		{access.RoleReceptionist, [3]access.Level{access.EDIT, access.VIEW, access.NONE}},
		{access.RoleManager, [3]access.Level{access.EDIT, access.VIEW, access.NONE}},
		{access.RoleOwner, [3]access.Level{access.EDIT, access.EDIT, access.EDIT}}, // implicit, even for a new building
	}
	for _, c := range cases {
		got, err := Stored{}.Levels(context.Background(), app.Caller{Role: c.role, Levels: stored}, ids)
		if err != nil || got["b1"] != c.want[0] || got["b2"] != c.want[1] || got["b3"] != c.want[2] {
			t.Errorf("%s: %v %v", c.role, got, err)
		}
	}
}
