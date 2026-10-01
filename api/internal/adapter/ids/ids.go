// Package ids generates prefixed ULID identifiers.
package ids

import (
	"crypto/rand"
	"time"
)

const (
	crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	timeChars = 10 // 48-bit millisecond timestamp
	randChars = 16 // 80 random bits
)

// Generator builds ids from an injected time source.
type Generator struct{ now func() time.Time }

// New returns a generator reading time from now.
func New(now func() time.Time) Generator { return Generator{now: now} }

// New returns prefix + "_" + a 26-character ULID. Ids from the same millisecond are unique by
// randomness, not ordered (ponytail: no monotonic counter; add one if ordering within a ms matters).
func (g Generator) New(prefix string) string {
	out := make([]byte, 0, len(prefix)+1+timeChars+randChars)
	out = append(out, prefix...)
	out = append(out, '_')
	ms := uint64(g.now().UnixMilli())
	var ts [timeChars]byte
	for i := timeChars - 1; i >= 0; i-- {
		ts[i] = crockford[ms&31]
		ms >>= 5
	}
	out = append(out, ts[:]...)
	rnd := make([]byte, randChars)
	_, _ = rand.Read(rnd) // Go 1.24+: crypto/rand.Read never returns an error (it aborts the process instead)
	for _, b := range rnd {
		out = append(out, crockford[b&31])
	}
	return string(out)
}
