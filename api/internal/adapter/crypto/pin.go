package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

// pinCost is bcrypt's work factor: about 250 ms per check, so a stolen hash table of six-digit PINs is slow to brute-force.
const pinCost = 12

// PinHasher hashes and checks PINs with bcrypt (a slow, salted hash).
type PinHasher struct{}

func (PinHasher) Hash(pin string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pin), pinCost)
	return string(h), err
}

// Verify is false for a wrong PIN or an unreadable hash.
func (PinHasher) Verify(hash, pin string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}

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
