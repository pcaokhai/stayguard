package app

// Encryptor protects sensitive fields at rest. Both calls fail closed: an input that cannot be
// authenticated is an error, never an empty value. The additional data binds the tenant and the
// field, so a ciphertext moved to another tenant or column does not open. No ctx: pure CPU.
type Encryptor interface {
	Encrypt(tenantID, field string, plaintext []byte) ([]byte, error)
	Decrypt(tenantID, field string, ciphertext []byte) ([]byte, error)
	// Fingerprint is a deterministic keyed digest, for request hashes that must not hold a
	// brute-forceable hash of a low-entropy secret.
	Fingerprint(tenantID, field string, value []byte) []byte
}
