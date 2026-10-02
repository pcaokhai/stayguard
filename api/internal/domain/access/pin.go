package access

import "errors"

// PinLength is the number of digits in a PIN.
const PinLength = 6

var (
	ErrPinFormat    = errors.New("pin must be six digits")
	ErrPinTooSimple = errors.New("pin is a run or a repeated digit")
)

// ValidatePinFormat accepts exactly PinLength ASCII digits.
func ValidatePinFormat(pin string) error {
	if len(pin) != PinLength {
		return ErrPinFormat
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return ErrPinFormat
		}
	}
	return nil
}

// ValidateNewPin also rejects a repeated digit (111111) and a run up or down (123456, 654321).
func ValidateNewPin(pin string) error {
	if err := ValidatePinFormat(pin); err != nil {
		return err
	}
	same, up, down := true, true, true
	for i := 1; i < len(pin); i++ {
		d := int(pin[i]) - int(pin[i-1])
		same = same && d == 0
		up = up && d == 1
		down = down && d == -1
	}
	if same || up || down {
		return ErrPinTooSimple
	}
	return nil
}
