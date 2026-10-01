package config

import "testing"

func TestRoomMapConfig_SG201_AC1(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": testDBURL}))
	if err != nil || c.RoomMapEnabled {
		t.Fatalf("default must be off: %+v err=%v", c, err)
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": testDBURL, "FF_S1_ROOM_MAP": "true"}))
	if err != nil || !c.RoomMapEnabled {
		t.Fatalf("true not applied: %+v err=%v", c, err)
	}
	if _, err = Load(env(map[string]string{"DATABASE_URL": testDBURL, "FF_S1_ROOM_MAP": "maybe"})); err == nil {
		t.Fatal("invalid value must fail startup")
	}
}
