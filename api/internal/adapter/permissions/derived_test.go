package permissions

import (
	"context"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

func TestDerivedLevels_SG201_AC1(t *testing.T) {
	ids := []string{"b1", "b2"}
	cases := map[access.Role]access.Level{
		access.RoleOwner: access.EDIT, access.RoleReceptionist: access.NONE, access.RoleHousekeeping: access.NONE,
	}
	for role, want := range cases {
		got, err := Derived{}.Levels(context.Background(), app.Caller{Role: role}, ids)
		if err != nil || len(got) != 2 || got["b1"] != want || got["b2"] != want {
			t.Errorf("%s: got %v err=%v, want %v for every building", role, got, err, want)
		}
	}
}
