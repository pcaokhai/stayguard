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
	ResultPartial   = "PARTIAL"
	ResultUnmatched = "UNMATCHED"
	// ResultIgnored is outgoing money or money to another account: stored for audit and dedupe, never listed and never linkable.
	ResultIgnored = "IGNORED"
	// ResultDismissed is an unmatched inbound transfer the owner closed with a note.
	ResultDismissed = "DISMISSED"
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

// FindBillCode returns the index of the code found inside the transfer content. Case and separators are ignored (banks add spaces,
// dashes and dots), but a match counts only when the character right after the code in the note as written is not a digit: PH1003A1012
// is another bill's code, not PH1003A101 followed by a 2. A separator, a letter or the end of the note after the code is fine. When
// one pending code is a prefix of another (PH0102A101 and PH0102A1012) the longest match wins.
func FindBillCode(content string, codes []string) (int, bool) {
	text, rawAt := normalizeWithPositions(content)
	raw := []rune(strings.ToUpper(content))
	best, bestLen := -1, 0
	for i, c := range codes {
		n := Normalize(c)
		if n == "" || len(n) <= bestLen {
			continue
		}
		for from := 0; ; {
			k := strings.Index(text[from:], n)
			if k < 0 {
				break
			}
			end := from + k + len(n) - 1 // index in text of the code's last character
			if next := rawAt[end] + 1; next >= len(raw) || raw[next] < '0' || raw[next] > '9' {
				best, bestLen = i, len(n)
				break
			}
			from += k + 1
		}
	}
	return best, best >= 0
}

// normalizeWithPositions is Normalize that also says where each kept character sits in the upper-cased content (in runes).
func normalizeWithPositions(s string) (string, []int) {
	var b strings.Builder
	var at []int
	for i, r := range []rune(strings.ToUpper(s)) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			at = append(at, i)
		}
	}
	return b.String(), at
}
