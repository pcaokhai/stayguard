package app

import "testing"

func TestRequestHash_IgnoresJSONWhitespaceAndKeyOrder_SG003_AC5(t *testing.T) {
	a := RequestHash([]byte(`{"a":1,"b":[1,2]}`))
	b := RequestHash([]byte("{ \"b\": [1, 2],\n \"a\": 1 }"))
	if a != b {
		t.Fatalf("equal JSON must hash equal: %s vs %s", a, b)
	}
	if a == RequestHash([]byte(`{"a":2,"b":[1,2]}`)) {
		t.Fatal("different JSON must hash differently")
	}
	if RequestHash([]byte(`9007199254740993`)) == RequestHash([]byte(`9007199254740992`)) {
		t.Fatal("large integers must not collapse through float64")
	}
	if RequestHash([]byte("not json")) == RequestHash([]byte("not  json")) {
		t.Fatal("non-JSON bodies hash as raw bytes")
	}
}
