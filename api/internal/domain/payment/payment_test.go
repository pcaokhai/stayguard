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

// parseTLV decodes an EMV QR string into its top-level fields; it fails on any length that does not fit.
func parseTLV(t *testing.T, s string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for len(s) > 0 {
		if len(s) < 4 {
			t.Fatalf("truncated field %q", s)
		}
		var n int
		if _, err := fmt.Sscanf(s[2:4], "%d", &n); err != nil || len(s) < 4+n {
			t.Fatalf("bad length in %q", s)
		}
		if _, dup := out[s[:2]]; dup {
			t.Fatalf("tag %s appears twice", s[:2])
		}
		out[s[:2]] = s[4 : 4+n]
		s = s[4+n:]
	}
	return out
}

// The payload a guest scans: a dynamic QR (tag 01 = 12) for exactly the amount, with the bill code as the transfer note and
// the one account passed in, decoded field by field rather than searched for as text.
func TestQRPayload_DecodedFields_DynamicAmountNoteAccount(t *testing.T) {
	const bin, account, note = "970436", "1017588888", "PH1002A101"
	p, err := QRPayload(bin, account, 210_000, note)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p[len(p)-4:], fmt.Sprintf("%04X", crc16(p[:len(p)-4])); got != want {
		t.Fatalf("checksum %s, want %s", got, want)
	}
	f := parseTLV(t, p)
	if f["00"] != "01" || f["01"] != "12" {
		t.Errorf("format %q, initiation %q: want 01 and 12 (dynamic: valid for this amount only)", f["00"], f["01"])
	}
	if f["54"] != "210000" || f["53"] != "704" || f["58"] != "VN" {
		t.Errorf("amount %q currency %q country %q", f["54"], f["53"], f["58"])
	}
	if extra := parseTLV(t, f["62"]); extra["08"] != note || len(extra) != 1 {
		t.Errorf("additional data %v: the bill code must be the transfer note (subfield 08) and nothing else", extra)
	}
	merchant := parseTLV(t, f["38"])
	member := parseTLV(t, merchant["01"])
	if merchant["00"] != "A000000727" || merchant["02"] != "QRIBFTTA" || member["00"] != bin || member["01"] != account || len(member) != 2 {
		t.Errorf("payee %v / %v: want NAPAS account transfer to %s at %s", merchant, member, account, bin)
	}
	if strings.Count(p, account) != 1 {
		t.Errorf("the account number must appear exactly once in %s", p)
	}
	// A static QR (tag 01 = 11) or one without an amount would let the guest type any sum: the builder cannot produce them.
	for _, amount := range []int64{0, -5} {
		if _, err := QRPayload(bin, account, amount, note); err == nil {
			t.Errorf("amount %d accepted", amount)
		}
	}
	other, _ := QRPayload(bin, account, 60_000, note)
	if parseTLV(t, other)["54"] != "60000" || other == p {
		t.Error("a different amount must give a different QR")
	}
}
