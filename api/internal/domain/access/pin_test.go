package access

import (
	"errors"
	"testing"
)

func TestValidateNewPin_SG701_AC3(t *testing.T) {
	for pin, want := range map[string]error{
		"482915": nil, "121212": nil, "123450": nil,
		"123456": ErrPinTooSimple, "654321": ErrPinTooSimple, "111111": ErrPinTooSimple, "012345": ErrPinTooSimple,
		"12345": ErrPinFormat, "1234567": ErrPinFormat, "12345a": ErrPinFormat, "": ErrPinFormat,
	} {
		if got := ValidateNewPin(pin); !errors.Is(got, want) {
			t.Errorf("%q: got %v want %v", pin, got, want)
		}
	}
}
