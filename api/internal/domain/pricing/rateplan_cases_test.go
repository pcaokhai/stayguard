package pricing

import (
	"encoding/json"
	"testing"
)

// planCase is shared by the Go validator test and the schema agreement test.
type planCase struct {
	name string
	doc  string
	want []FieldError // nil means the plan is valid
}

func validMap() map[string]any {
	return map[string]any{
		"version":      2,
		"currency":     "VND",
		"graceMinutes": 15,
		"hourly":       map[string]any{"firstHour": 80000, "extraHour": 20000},
		"overnight":    map[string]any{"price": 250000, "windowStart": "21:00", "windowEnd": "12:00"},
		"daily":        map[string]any{"price": 350000, "windowStart": "14:00", "windowEnd": "12:00"},
	}
}

// edit applies changes to a valid plan: a nil value deletes the dotted path.
func edit(t *testing.T, changes map[string]any) string {
	t.Helper()
	m := validMap()
	for path, v := range changes {
		segs := splitPath(path)
		parent := m
		for _, seg := range segs[:len(segs)-1] {
			parent = parent[seg].(map[string]any)
		}
		key := segs[len(segs)-1]
		if v == nil {
			delete(parent, key)
		} else {
			parent[key] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func splitPath(p string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '.' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	return out
}

func fe(path, code string) FieldError { return FieldError{Path: path, Code: code} }

func planCases(t *testing.T) []planCase {
	t.Helper()
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	cs := []planCase{
		{"valid", edit(t, nil), nil},
		{"valid zero money and grace", edit(t, map[string]any{"graceMinutes": 0, "hourly.firstHour": 0}), nil},
		{"valid grace 60", edit(t, map[string]any{"graceMinutes": 60}), nil},
		{"valid clocks 00:00 and 23:59", edit(t, map[string]any{"overnight.windowStart": "00:00", "daily.windowEnd": "23:59"}), nil},

		{"wrong type version", edit(t, map[string]any{"version": "2"}), []FieldError{fe("version", "TYPE")}},
		{"wrong type price", edit(t, map[string]any{"daily.price": "350000"}), []FieldError{fe("daily.price", "TYPE")}},
		{"wrong type object", edit(t, map[string]any{"hourly": 5}), []FieldError{fe("hourly", "TYPE")}},
		{"wrong type clock", edit(t, map[string]any{"daily.windowStart": 14}), []FieldError{fe("daily.windowStart", "TYPE")}},
		{"wrong type grace", edit(t, map[string]any{"graceMinutes": true}), []FieldError{fe("graceMinutes", "TYPE")}},
		{"null nested object", edit(t, map[string]any{"overnight": raw("null")}), []FieldError{fe("overnight", "TYPE")}},

		{"negative money", edit(t, map[string]any{"hourly.firstHour": -1}), []FieldError{fe("hourly.firstHour", "MIN")}},
		{"fractional money", edit(t, map[string]any{"hourly.extraHour": raw("80000.5")}), []FieldError{fe("hourly.extraHour", "TYPE")}},
		{"fractional grace", edit(t, map[string]any{"graceMinutes": raw("1.5")}), []FieldError{fe("graceMinutes", "TYPE")}},
		{"fractional version", edit(t, map[string]any{"version": raw("1.5")}), []FieldError{fe("version", "TYPE")}},
		{"grace 61", edit(t, map[string]any{"graceMinutes": 61}), []FieldError{fe("graceMinutes", "MAX")}},
		{"grace -1", edit(t, map[string]any{"graceMinutes": -1}), []FieldError{fe("graceMinutes", "MIN")}},
		{"version 0", edit(t, map[string]any{"version": 0}), []FieldError{fe("version", "MIN")}},
		{"currency USD", edit(t, map[string]any{"currency": "USD"}), []FieldError{fe("currency", "CONST")}},
		{"currency number", edit(t, map[string]any{"currency": 1}), []FieldError{fe("currency", "CONST")}},

		{"clock 24:00", edit(t, map[string]any{"overnight.windowStart": "24:00"}), []FieldError{fe("overnight.windowStart", "PATTERN")}},
		{"clock 9:00", edit(t, map[string]any{"overnight.windowEnd": "9:00"}), []FieldError{fe("overnight.windowEnd", "PATTERN")}},
		{"clock 12:60", edit(t, map[string]any{"daily.windowStart": "12:60"}), []FieldError{fe("daily.windowStart", "PATTERN")}},
		{"clock empty", edit(t, map[string]any{"daily.windowEnd": ""}), []FieldError{fe("daily.windowEnd", "PATTERN")}},
		{"clock trailing newline", edit(t, map[string]any{"daily.windowEnd": "12:00\n"}), []FieldError{fe("daily.windowEnd", "PATTERN")}},

		{"extra top level", edit(t, map[string]any{"extra": 1}), []FieldError{fe("extra", "UNKNOWN_PROPERTY")}},
		{"extra nested", edit(t, map[string]any{"hourly.extra": 1}), []FieldError{fe("hourly.extra", "UNKNOWN_PROPERTY")}},

		{"several errors together", edit(t, map[string]any{
			"version": 0, "graceMinutes": 61, "currency": "USD", "hourly.firstHour": -5, "extra": 1, "daily.windowEnd": "24:00", "overnight.price": nil,
		}), []FieldError{
			fe("currency", "CONST"), fe("daily.windowEnd", "PATTERN"), fe("extra", "UNKNOWN_PROPERTY"),
			fe("graceMinutes", "MAX"), fe("hourly.firstHour", "MIN"), fe("overnight.price", "REQUIRED"), fe("version", "MIN"),
		}},

		{"empty object", `{}`, []FieldError{
			fe("currency", "REQUIRED"), fe("daily", "REQUIRED"), fe("graceMinutes", "REQUIRED"),
			fe("hourly", "REQUIRED"), fe("overnight", "REQUIRED"), fe("version", "REQUIRED"),
		}},
		{"json null", `null`, []FieldError{fe("", "TYPE")}},
		{"json array", `[]`, []FieldError{fe("", "TYPE")}},
		{"empty nested objects", `{"version":1,"currency":"VND","graceMinutes":0,"hourly":{},"overnight":{},"daily":{}}`, []FieldError{
			fe("daily.price", "REQUIRED"), fe("daily.windowEnd", "REQUIRED"), fe("daily.windowStart", "REQUIRED"),
			fe("hourly.extraHour", "REQUIRED"), fe("hourly.firstHour", "REQUIRED"),
			fe("overnight.price", "REQUIRED"), fe("overnight.windowEnd", "REQUIRED"), fe("overnight.windowStart", "REQUIRED"),
		}},
	}
	for _, p := range []string{"version", "currency", "graceMinutes", "hourly", "overnight", "daily",
		"hourly.firstHour", "hourly.extraHour", "overnight.price", "overnight.windowStart", "overnight.windowEnd",
		"daily.price", "daily.windowStart", "daily.windowEnd"} {
		cs = append(cs, planCase{"missing " + p, edit(t, map[string]any{p: nil}), []FieldError{fe(p, "REQUIRED")}})
	}
	return cs
}

// malformedCases are not JSON at all, so the schema file cannot judge them.
var malformedCases = []string{``, `{`, `{"version":1,}`, `not json`, `{} {}`, `{"version":1} x`}

// strictIntegerCases are mathematically integers (the schema accepts them) but written
// as fractions or exponents; the Go validator rejects the literal form for money.
func strictIntegerCases(t *testing.T) []planCase {
	t.Helper()
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	return []planCase{
		{"money 80000.0", edit(t, map[string]any{"hourly.firstHour": raw("80000.0")}), []FieldError{fe("hourly.firstHour", "TYPE")}},
		{"money 8e4", edit(t, map[string]any{"hourly.firstHour": raw("8e4")}), []FieldError{fe("hourly.firstHour", "TYPE")}},
		{"grace 1e1", edit(t, map[string]any{"graceMinutes": raw("1e1")}), []FieldError{fe("graceMinutes", "TYPE")}},
		{"version 2.0", edit(t, map[string]any{"version": raw("2.0")}), []FieldError{fe("version", "TYPE")}},
	}
}
