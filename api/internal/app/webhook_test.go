package app

import "testing"

// The vector comes from an independent HMAC-SHA256 of "{timestamp}.{raw body}", as SePay's documentation specifies.
func TestSepaySignature_Vector_SG703(t *testing.T) {
	got := SepaySignature([]byte("s3cret"), "1700000000", []byte(`{"id":1}`))
	if got != "sha256=ee0658aa4e37018df69c24227df01e0f680eb3b87c7f1f9bd936e283cfe01d9b" {
		t.Fatalf("got %s", got)
	}
}
