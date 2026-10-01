package pricing

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

func TestRatePlanSchema_SG101_AC2(t *testing.T) {
	cases := append(planCases(t), strictIntegerCases(t)...)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, err := ParseRatePlan([]byte(c.doc))
			if c.want == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *ValidationError, got %v (plan %+v)", err, plan)
			}
			if !reflect.DeepEqual(ve.Errors, c.want) {
				t.Fatalf("errors\n got  %v\n want %v", ve.Errors, c.want)
			}
		})
	}
}

func TestRatePlanParsedValues_SG101_AC2(t *testing.T) {
	plan, err := ParseRatePlan([]byte(edit(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	want := RatePlan{
		Version: 2, Currency: CurrencyVND, GraceMinutes: 15,
		Hourly:    Hourly{FirstHour: money.Vnd(80000), ExtraHour: money.Vnd(20000)},
		Overnight: Window{Price: money.Vnd(250000), Start: Clock{21, 0}, End: Clock{12, 0}},
		Daily:     Window{Price: money.Vnd(350000), Start: Clock{14, 0}, End: Clock{12, 0}},
	}
	if plan != want {
		t.Fatalf("got %+v want %+v", plan, want)
	}
}

func TestRatePlanMalformedJSON_SG101_AC2(t *testing.T) {
	for _, doc := range malformedCases {
		_, err := ParseRatePlan([]byte(doc))
		var ve *ValidationError
		if !errors.As(err, &ve) || !reflect.DeepEqual(ve.Errors, []FieldError{fe("", "INVALID_JSON")}) {
			t.Errorf("%q: got %v", doc, err)
		}
	}
}

func TestRatePlanErrorsHideInput_SG101_AC2(t *testing.T) {
	const marker = "SECRET-MARKER"
	docs := []string{
		edit(t, map[string]any{"currency": marker}),
		edit(t, map[string]any{"daily.windowStart": marker}),
		edit(t, map[string]any{"version": marker}),
		`{"version":` + `"` + marker + `"`, // malformed, value inside
	}
	for _, doc := range docs {
		_, err := ParseRatePlan([]byte(doc))
		if err == nil {
			t.Fatalf("want error for %s", doc)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("error text leaks input: %v", err)
		}
		var ve *ValidationError
		if errors.As(err, &ve) && strings.Contains(strings.Join(codesAndPaths(ve.Errors), " "), marker) {
			t.Errorf("field errors leak input: %v", ve.Errors)
		}
	}
}

func TestRatePlanUnknownKeysNotEchoed_SG101_AC2(t *testing.T) {
	const marker = "SECRET-MARKER"
	keys := []string{marker, strings.Repeat("k", 64*1024), "a\x1b[31m\x00b\n", "hourly.firstHour"}
	for _, key := range keys {
		m := validMap()
		m[key] = 1
		m["hourly"].(map[string]any)[key+"2"] = 1
		m[key+"3"] = 1
		doc, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ParseRatePlan(doc)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("want *ValidationError, got %v", err)
		}
		want := []FieldError{fe("", "UNKNOWN_PROPERTY"), fe("hourly", "UNKNOWN_PROPERTY")}
		if !reflect.DeepEqual(ve.Errors, want) {
			t.Errorf("key %.12q: got %v want %v", key, ve.Errors, want)
		}
		if len(err.Error()) > 200 || strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), "\x1b") {
			t.Errorf("error text unbounded or echoes key: %.200q", err.Error())
		}
	}
}

func codesAndPaths(es []FieldError) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Path, e.Code)
	}
	return out
}
