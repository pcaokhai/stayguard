package stay

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// ErrNotActive: the stay is not ACTIVE, so extras cannot be added (HTTP 409 STAY_NOT_ACTIVE).
var ErrNotActive = errors.New("stay is not active")

// ErrInsufficientStock: a service has less stock than requested (HTTP 409 INSUFFICIENT_STOCK).
var ErrInsufficientStock = errors.New("insufficient stock")

// CodeUnknown marks a service code the tenant's catalogue does not have.
const CodeUnknown = "UNKNOWN"

// NewValidationError builds the same sorted error ValidateExtras returns, for checks that need the catalogue.
func NewValidationError(errs []FieldError) *ValidationError { return newValidationError(errs) }

const (
	pathItems       = "items"
	minExtraQty     = 1
	maxExtraQty     = 99
	maxExtraItems   = 50
	maxServiceCodes = 64
)

// ExtraInput is one raw item of an add-extras request.
type ExtraInput struct {
	ServiceCode string
	Quantity    int
}

// ExtraLine is a validated, merged item: one line per service code.
type ExtraLine struct {
	ServiceCode string
	Quantity    int
}

// RequireActive fails closed: anything but ACTIVE is not active.
func RequireActive(status Status) error {
	if status != StatusActive {
		return ErrNotActive
	}
	return nil
}

// ValidateExtras checks every item, merges duplicate codes (the merged quantity stays within the
// quantity bounds) and returns the lines sorted by code, so stock rows are always locked in one order.
func ValidateExtras(items []ExtraInput) ([]ExtraLine, error) {
	if len(items) == 0 {
		return nil, newValidationError([]FieldError{{pathItems, CodeRequired}})
	}
	if len(items) > maxExtraItems {
		return nil, newValidationError([]FieldError{{pathItems, CodeMax}})
	}
	var errs []FieldError
	merged := map[string]int{}
	for i, it := range items {
		code := strings.TrimSpace(it.ServiceCode)
		fes := append(checkServiceCode(i, code), checkQuantity(i, it.Quantity)...)
		if len(fes) == 0 && merged[code]+it.Quantity > maxExtraQty {
			fes = []FieldError{{fmt.Sprintf("items[%d].quantity", i), CodeMax}}
		}
		if errs = append(errs, fes...); len(fes) == 0 {
			merged[code] += it.Quantity
		}
	}
	if len(errs) > 0 {
		return nil, newValidationError(errs)
	}
	lines := make([]ExtraLine, 0, len(merged))
	for code, q := range merged {
		lines = append(lines, ExtraLine{code, q})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ServiceCode < lines[j].ServiceCode })
	return lines, nil
}

func checkServiceCode(i int, code string) []FieldError {
	path := fmt.Sprintf("items[%d].serviceCode", i)
	switch {
	case code == "":
		return []FieldError{{path, CodeRequired}}
	case utf8.RuneCountInString(code) > maxServiceCodes:
		return []FieldError{{path, CodeTooLong}}
	}
	for _, r := range code {
		if !isASCIIAlnum(r) && r != '_' && r != '-' {
			return []FieldError{{path, CodePattern}}
		}
	}
	return nil
}

func checkQuantity(i, q int) []FieldError {
	path := fmt.Sprintf("items[%d].quantity", i)
	switch {
	case q < minExtraQty:
		return []FieldError{{path, CodeMin}}
	case q > maxExtraQty:
		return []FieldError{{path, CodeMax}}
	}
	return nil
}
