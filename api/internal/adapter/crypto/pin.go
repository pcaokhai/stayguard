package crypto

import "golang.org/x/crypto/bcrypt"

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
