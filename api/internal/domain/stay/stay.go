// Package stay holds the pure rules of a guest stay: status and check-in input validation.
package stay

import (
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

type Status string

const (
	StatusActive     Status = "ACTIVE"
	StatusCheckedOut Status = "CHECKED_OUT"
)

// ParseStatus fails closed on anything but the two stored statuses.
func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case StatusActive, StatusCheckedOut:
		return st, nil
	}
	return "", errors.New("stay: unknown status")
}

const (
	CodeRequired = "REQUIRED"
	CodeTooShort = "TOO_SHORT"
	CodeTooLong  = "TOO_LONG"
	CodePattern  = "PATTERN"
	CodeMin      = "MIN"
	CodeMax      = "MAX"
)

const (
	pathName    = "guestName"
	pathPhone   = "guestPhone"
	pathID      = "idNumber"
	pathDeposit = "deposit"
)

const (
	maxNameRunes = 120
	minPhoneLen  = 6
	maxPhoneLen  = 20
	minPhoneDigs = 6
	minIDLen     = 5
	maxIDLen     = 20
	// MaxDeposit guards against overflow and typos (an extra zero); 100 million VND.
	MaxDeposit int64 = 100_000_000
	// maskedPrefix has a fixed length so the id number length is not revealed.
	maskedPrefix = "*****"
	maskedTail   = 3
	// minMaskTailLen: below this, revealing 3 characters would show too much of the id.
	minMaskTailLen = 8
)

// FieldError never carries the offending value: it can quote client data.
type FieldError struct {
	Path string
	Code string
}

type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.Path + " " + fe.Code
	}
	return "invalid stay input: " + strings.Join(parts, "; ")
}

// Guest holds the normalized (trimmed) values. IDNumber is empty when none was given.
type Guest struct {
	Name, Phone, IDNumber string
}

// ValidateGuest reports every problem at once, sorted by path then code.
func ValidateGuest(name, phone, idNumber string) (Guest, error) {
	g := Guest{Name: strings.TrimSpace(name), Phone: strings.TrimSpace(phone), IDNumber: strings.TrimSpace(idNumber)}
	var errs []FieldError
	errs = append(errs, checkName(g.Name)...)
	errs = append(errs, checkPhone(g.Phone)...)
	errs = append(errs, checkID(g.IDNumber)...)
	if len(errs) > 0 {
		return Guest{}, newValidationError(errs)
	}
	return g, nil
}

// ValidateDeposit bounds the deposit in whole VND.
func ValidateDeposit(n int64) (money.Vnd, error) {
	switch {
	case n < 0:
		return 0, newValidationError([]FieldError{{pathDeposit, CodeMin}})
	case n > MaxDeposit:
		return 0, newValidationError([]FieldError{{pathDeposit, CodeMax}})
	}
	return money.NewVnd(n)
}

func newValidationError(errs []FieldError) *ValidationError {
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		return errs[i].Code < errs[j].Code
	})
	return &ValidationError{Errors: errs}
}

func checkName(s string) []FieldError {
	switch n := utf8.RuneCountInString(s); {
	case n == 0:
		return []FieldError{{pathName, CodeRequired}}
	case n > maxNameRunes:
		return []FieldError{{pathName, CodeTooLong}}
	}
	if strings.IndexFunc(s, isHiddenRune) >= 0 {
		return []FieldError{{pathName, CodePattern}}
	}
	return nil
}

// isHiddenRune: control, format (bidi override, zero width) and line or paragraph separators
// can spoof or hide text on a screen or in a log.
func isHiddenRune(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

func checkPhone(s string) []FieldError {
	switch n := utf8.RuneCountInString(s); {
	case n == 0:
		return []FieldError{{pathPhone, CodeRequired}}
	case n < minPhoneLen:
		return []FieldError{{pathPhone, CodeTooShort}}
	case n > maxPhoneLen:
		return []FieldError{{pathPhone, CodeTooLong}}
	}
	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		case r == '+' && i == 0:
		default:
			return []FieldError{{pathPhone, CodePattern}}
		}
	}
	if digits < minPhoneDigs {
		return []FieldError{{pathPhone, CodeTooShort}}
	}
	return nil
}

func checkID(s string) []FieldError {
	if s == "" {
		return nil
	}
	var errs []FieldError
	n := utf8.RuneCountInString(s)
	if n < minIDLen {
		errs = append(errs, FieldError{pathID, CodeTooShort})
	}
	if n > maxIDLen {
		errs = append(errs, FieldError{pathID, CodeTooLong})
	}
	for _, r := range s {
		if !isASCIIAlnum(r) {
			return append(errs, FieldError{pathID, CodePattern})
		}
	}
	return errs
}

func isASCIIAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// MaskIDNumber shows only the last 3 characters; short ids are fully masked.
func MaskIDNumber(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) < minMaskTailLen {
		return maskedPrefix
	}
	return maskedPrefix + string(r[len(r)-maskedTail:])
}
