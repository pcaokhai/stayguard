package config

import (
	"fmt"
	"strconv"
)

// loadRoomMap reads the room map slice flag (SG-201): FF_S1_ROOM_MAP, default off.
func loadRoomMap(c Config, getenv func(string) string) (Config, error) {
	if v := getenv("FF_S1_ROOM_MAP"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("FF_S1_ROOM_MAP must be a boolean, got %q", v)
		}
		c.RoomMapEnabled = b
	}
	return c, nil
}
