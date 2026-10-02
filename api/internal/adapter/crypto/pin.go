package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// pinCost is bcrypt's work factor: about 250 ms per check, so a stolen hash table of six-digit PINs is slow to brute-force.
const pinCost = 12

// pepperLabel separates the PIN pepper from every other use of the data key.
const pepperLabel = "stayguard/pin-pepper/v1"

// pepperedPrefix marks a hash made with the pepper; a hash without it is a legacy plain bcrypt hash.
const pepperedPrefix = "p1$"

// DerivePinPepper is the server-side secret mixed into every PIN before bcrypt: HMAC-SHA256 of a fixed label keyed
// by the data encryption key. A stolen database without the key cannot be brute-forced offline.
func DerivePinPepper(dataKey []byte) []byte {
	m := hmac.New(sha256.New, dataKey)
	m.Write([]byte(pepperLabel))
	return m.Sum(nil)
}

// PinHasher hashes and checks PINs: bcrypt over the HMAC-SHA256 of the PIN keyed by the pepper.
// Hashes made before the pepper existed (plain bcrypt) still verify and report NeedsRehash.
type PinHasher struct{ pepper []byte }

// NewPinHasher builds the hasher from the data encryption key.
func NewPinHasher(dataKey []byte) PinHasher { return PinHasher{pepper: DerivePinPepper(dataKey)} }

// mixed is the bcrypt input: 64 hex characters, inside bcrypt's 72-byte limit.
func (h PinHasher) mixed(pin string) []byte {
	m := hmac.New(sha256.New, h.pepper)
	m.Write([]byte(pin))
	return []byte(hex.EncodeToString(m.Sum(nil)))
}

func (h PinHasher) Hash(pin string) (string, error) {
	b, err := bcrypt.GenerateFromPassword(h.mixed(pin), pinCost)
	return pepperedPrefix + string(b), err
}

// Verify is false for a wrong PIN or an unreadable hash.
func (h PinHasher) Verify(hash, pin string) bool {
	if rest, ok := strings.CutPrefix(hash, pepperedPrefix); ok {
		return bcrypt.CompareHashAndPassword([]byte(rest), h.mixed(pin)) == nil
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}

// NeedsRehash is true for a legacy hash without the pepper.
func (PinHasher) NeedsRehash(hash string) bool { return !strings.HasPrefix(hash, pepperedPrefix) }

// maxPinDraws bounds the redraw loop; a draw fails the PIN rules about once in 10^5.
const maxPinDraws = 20

// PinGenerator draws one-time PINs from crypto/rand, skipping runs and repeated digits.
type PinGenerator struct{}

func (PinGenerator) New() (string, error) {
	for range maxPinDraws {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			return "", err
		}
		if pin := fmt.Sprintf("%06d", n.Int64()); access.ValidateNewPin(pin) == nil {
			return pin, nil
		}
	}
	return "", errors.New("could not draw a valid pin")
}
