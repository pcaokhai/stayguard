package payment

import (
	"fmt"
	"strings"
	"testing"
)

func TestCRC16KnownSample_A2(t *testing.T) {
	if got := crc16("123456789"); got != 0x29B1 { // the standard CCITT-FALSE check value
		t.Fatalf("crc16 = %04X, want 29B1", got)
	}
}

func TestQRPayload_A2(t *testing.T) {
	p, err := QRPayload("970000", "0000000000", 123000, "PH0102A101")
	if err != nil {
		t.Fatal(err)
	}
	body, sum := p[:len(p)-4], p[len(p)-4:]
	if want := fmt.Sprintf("%04X", crc16(body)); sum != want || !strings.HasSuffix(body, "6304") {
		t.Fatalf("checksum %s, want %s in %s", sum, want, p)
	}
	for _, part := range []string{"000201", "010212", "5406123000", "5303704", "5802VN", "0810PH0102A101", "0010A000000727", "0006970000", "0110" + "0000000000", "0208QRIBFTTA"} {
		if !strings.Contains(p, part) {
			t.Errorf("payload %s misses %s", p, part)
		}
	}
}

func TestQRPayloadRejects_A2(t *testing.T) {
	for _, c := range []struct {
		bin, acc string
		amt      int64
		note     string
	}{{"", "1", 1, "x"}, {"1", "", 1, "x"}, {"1", "1", 0, "x"}, {"1", "1", 1, ""}} {
		if _, err := QRPayload(c.bin, c.acc, c.amt, c.note); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
}

func TestFindBillCode_A2(t *testing.T) {
	codes := []string{"PH0102A101", "PH0102A1012"}
	for content, want := range map[string]int{
		"ph0102a101":                 0,
		"CK PH-0102 A101 tien phong": 0,
		"ph 0102a1012 thanh toan":    1, // longest wins
	} {
		if i, ok := FindBillCode(content, codes); !ok || i != want {
			t.Errorf("%q = %d,%v want %d", content, i, ok, want)
		}
	}
	if _, ok := FindBillCode("hello", codes); ok {
		t.Error("matched unrelated content")
	}
}
