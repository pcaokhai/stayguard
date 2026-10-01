package stay

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const extrasMarker = "MARKER_CODE"

func extrasErrs(t *testing.T, items []ExtraInput) []FieldError {
	t.Helper()
	_, err := ValidateExtras(items)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	return ve.Errors
}

func TestExtrasValidation_SG205_AC2(t *testing.T) {
	t.Run("quantity boundaries", func(t *testing.T) {
		for _, q := range []int{1, 99} {
			if _, err := ValidateExtras([]ExtraInput{{"WATER", q}}); err != nil {
				t.Fatalf("quantity %d: %v", q, err)
			}
		}
		for _, c := range []struct {
			q    int
			code string
		}{{0, CodeMin}, {-1, CodeMin}, {100, CodeMax}} {
			got := extrasErrs(t, []ExtraInput{{"WATER", c.q}})
			if !reflect.DeepEqual(got, []FieldError{{"items[0].quantity", c.code}}) {
				t.Fatalf("quantity %d: %v", c.q, got)
			}
		}
	})
	t.Run("item count", func(t *testing.T) {
		if got := extrasErrs(t, nil); !reflect.DeepEqual(got, []FieldError{{"items", CodeRequired}}) {
			t.Fatalf("empty: %v", got)
		}
		many := make([]ExtraInput, 51)
		for i := range many {
			many[i] = ExtraInput{string(rune('A'+i/26)) + string(rune('A'+i%26)), 1}
		}
		if got := extrasErrs(t, many); !reflect.DeepEqual(got, []FieldError{{"items", CodeMax}}) {
			t.Fatalf("51 items: %v", got)
		}
		if _, err := ValidateExtras(many[:50]); err != nil {
			t.Fatalf("50 items: %v", err)
		}
	})
	t.Run("codes", func(t *testing.T) {
		long := strings.Repeat("A", 65)
		for _, c := range []struct{ in, code string }{
			{"", CodeRequired}, {"   ", CodeRequired}, {long, CodeTooLong}, {"WA TER", CodePattern}, {"NƯỚC", CodePattern}, {"A/B", CodePattern},
		} {
			got := extrasErrs(t, []ExtraInput{{c.in, 1}})
			if !reflect.DeepEqual(got, []FieldError{{"items[0].serviceCode", c.code}}) {
				t.Fatalf("code %q: %v", c.in, got)
			}
		}
		if out, err := ValidateExtras([]ExtraInput{{" " + strings.Repeat("a", 64) + " ", 1}}); err != nil || len(out[0].ServiceCode) != 64 {
			t.Fatalf("64 chars, trimmed: %v %v", out, err)
		}
		if _, err := ValidateExtras([]ExtraInput{{"a_B-9", 1}}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("merge and order", func(t *testing.T) {
		out, err := ValidateExtras([]ExtraInput{{"WATER", 2}, {"BEER", 1}, {" WATER ", 3}})
		want := []ExtraLine{{"BEER", 1}, {"WATER", 5}}
		if err != nil || !reflect.DeepEqual(out, want) {
			t.Fatalf("got %v %v", out, err)
		}
	})
	t.Run("merge overflow", func(t *testing.T) {
		got := extrasErrs(t, []ExtraInput{{"WATER", 60}, {"BEER", 1}, {"WATER", 40}})
		if !reflect.DeepEqual(got, []FieldError{{"items[2].quantity", CodeMax}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("all errors at once, no echo", func(t *testing.T) {
		got := extrasErrs(t, []ExtraInput{{"", 0}, {extrasMarker + " x", 100}})
		want := []FieldError{{"items[0].quantity", CodeMin}, {"items[0].serviceCode", CodeRequired},
			{"items[1].quantity", CodeMax}, {"items[1].serviceCode", CodePattern}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v", got)
		}
		_, err := ValidateExtras([]ExtraInput{{extrasMarker + " x", 1}})
		if strings.Contains(err.Error(), extrasMarker) {
			t.Fatalf("error echoes input: %v", err)
		}
	})
}

func TestRequireActive_SG205_AC4(t *testing.T) {
	if err := RequireActive(StatusActive); err != nil {
		t.Fatal(err)
	}
	if err := RequireActive(StatusCheckedOut); !errors.Is(err, ErrNotActive) {
		t.Fatalf("got %v", err)
	}
	if err := RequireActive(Status("")); !errors.Is(err, ErrNotActive) {
		t.Fatalf("unknown status must fail closed: %v", err)
	}
	if errors.Is(ErrNotActive, ErrInsufficientStock) {
		t.Fatal("errors must be distinct")
	}
}
