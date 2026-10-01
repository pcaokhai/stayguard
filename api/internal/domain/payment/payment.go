// Package payment holds the pure payment rules: methods, statuses, bill-code matching and the VietQR payload.
package payment

import (
	"errors"
	"strings"
)

const (
	MethodCash     = "CASH"
	MethodTransfer = "TRANSFER"

	StatusPending  = "PENDING"
	StatusPaid     = "PAID"
	StatusExpired  = "EXPIRED"
	StatusMismatch = "MISMATCH"

	ResultSettled   = "SETTLED"
	ResultMismatch  = "MISMATCH"
	ResultUnmatched = "UNMATCHED"
	ResultDuplicate = "DUPLICATE_IGNORED"
)

var ErrUnknownMethod = errors.New("unknown payment method")

func ParseMethod(s string) (string, error) {
	if s == MethodCash || s == MethodTransfer {
		return s, nil
	}
	return "", ErrUnknownMethod
}

// Normalize upper-cases and keeps ASCII letters and digits only: banks often change case and add
// spaces, dashes or dots to the transfer note.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindBillCode returns the index of the code found inside the transfer content. When one code is a prefix
// of another (PH0102A101 and PH0102A1012) the longest match wins.
func FindBillCode(content string, codes []string) (int, bool) {
	text, best, bestLen := Normalize(content), -1, 0
	for i, c := range codes {
		n := Normalize(c)
		if n != "" && len(n) > bestLen && strings.Contains(text, n) {
			best, bestLen = i, len(n)
		}
	}
	return best, best >= 0
}
