package stay

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

const marker = "ZZMARKERZZ"

func codes(err error) []FieldError {
	var ve *ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	return ve.Errors
}

func TestStayValidation_SG203_AC1(t *testing.T) {
	cases := []struct {
		name          string
		guest, ph, id string
		want          []FieldError
		wantGuest     Guest
	}{
		{"valid all", "  Nguyen An ", " +84 90 123-4567 ", " AB12345 ", nil, Guest{"Nguyen An", "+84901234567", "AB12345"}},
		{"valid no id", "A", "0901234567", "", nil, Guest{"A", "0901234567", ""}},
		{"id only spaces is none", "A", "0901234567", "   ", nil, Guest{"A", "0901234567", ""}},
		{"name 120 runes", strings.Repeat("a", 120), "0901234567", "", nil, Guest{strings.Repeat("a", 120), "0901234567", ""}},
		{"name 120 multibyte", strings.Repeat("ễ", 120), "0901234567", "", nil, Guest{strings.Repeat("ễ", 120), "0901234567", ""}},
		{"name 121 runes", strings.Repeat("ễ", 121), "0901234567", "", []FieldError{{"guestName", "TOO_LONG"}}, Guest{}},
		{"name empty", "", "0901234567", "", []FieldError{{"guestName", "REQUIRED"}}, Guest{}},
		{"name spaces only", "   ", "0901234567", "", []FieldError{{"guestName", "REQUIRED"}}, Guest{}},
		{"name control", "a\x07b", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name newline inside", "a\nb", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name bidi override", "a\u202eb", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name zero width", "a\u200bb", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name only zero width", "\u200b\u200b", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name line separator inside", "a\u2028b", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name paragraph separator inside", "a\u2029b", "0901234567", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name only line separator", "\u2028", "0901234567", "", []FieldError{{"guestName", "REQUIRED"}}, Guest{}},
		{"vietnamese precomposed", "Nguy\u1ec5n V\u0103n \u00c1nh", "0901234567", "", nil, Guest{"Nguy\u1ec5n V\u0103n \u00c1nh", "0901234567", ""}},
		{"vietnamese decomposed", "Nguye\u0303n Va\u0306n A\u0301nh", "0901234567", "", nil, Guest{"Nguye\u0303n Va\u0306n A\u0301nh", "0901234567", ""}},
		{"phone empty", "A", "", "", []FieldError{{"guestPhone", "REQUIRED"}}, Guest{}},
		{"phone 8 digits", "A", "09012345", "", []FieldError{{"guestPhone", "TOO_SHORT"}}, Guest{}},
		{"phone 9 digits", "A", "901234567", "", nil, Guest{"A", "901234567", ""}},
		{"phone 10 digits with a leading 0", "A", "0901234567", "", nil, Guest{"A", "0901234567", ""}},
		{"phone 11 digits", "A", "09012345678", "", nil, Guest{"A", "09012345678", ""}},
		{"phone 12 digits", "A", "090123456789", "", []FieldError{{"guestPhone", "TOO_LONG"}}, Guest{}},
		{"phone +84 with 9 digits after", "A", "+84901234567", "", nil, Guest{"A", "+84901234567", ""}},
		{"phone +84 too long", "A", "+849012345678", "", []FieldError{{"guestPhone", "TOO_LONG"}}, Guest{}},
		{"phone another country code", "A", "+11234567890", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"phone spaces dots dashes are stripped", "A", "090 123.45-67", "", nil, Guest{"A", "0901234567", ""}},
		{"phone +84 with separators is stripped", "A", "+84 901-234.567", "", nil, Guest{"A", "+84901234567", ""}},
		{"phone 8 digits with separators", "A", "090 1234-5", "", []FieldError{{"guestPhone", "TOO_SHORT"}}, Guest{}},
		{"phone brackets are not allowed", "A", "(090)1234567", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"phone plus in the middle", "A", "090+1234567", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"phone letters", "A", "09012a4567", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"id 4", "A", "0901234567", "AB12", []FieldError{{"idNumber", "TOO_SHORT"}}, Guest{}},
		{"id 5", "A", "0901234567", "AB123", nil, Guest{"A", "0901234567", "AB123"}},
		{"id 20", "A", "0901234567", strings.Repeat("9", 20), nil, Guest{"A", "0901234567", strings.Repeat("9", 20)}},
		{"id 21", "A", "0901234567", strings.Repeat("9", 21), []FieldError{{"idNumber", "TOO_LONG"}}, Guest{}},
		{"id non ascii", "A", "0901234567", "ABCDÉ", []FieldError{{"idNumber", "PATTERN"}}, Guest{}},
		{"id symbols", "A", "0901234567", "AB-123", []FieldError{{"idNumber", "PATTERN"}}, Guest{}},
		{"several at once", "", "1", "!", []FieldError{{"guestName", "REQUIRED"}, {"guestPhone", "TOO_SHORT"}, {"idNumber", "PATTERN"}, {"idNumber", "TOO_SHORT"}}, Guest{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, err := ValidateGuest(c.guest, c.ph, c.id)
			if c.want == nil {
				if err != nil || g != c.wantGuest {
					t.Fatalf("got %+v err=%v, want %+v", g, err, c.wantGuest)
				}
				return
			}
			if got := codes(err); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("errors = %v, want %v", got, c.want)
			}
		})
	}
}

func TestStayValidationNeverEchoes_SG203_AC1(t *testing.T) {
	_, err := ValidateGuest(marker+"\x07", marker+"x", marker+"-")
	if err == nil || strings.Contains(err.Error(), marker) {
		t.Fatalf("error must exist and not contain input: %v", err)
	}
	if _, err = ValidateDeposit(-1); err == nil {
		t.Fatal("want error")
	}
}

func TestDepositBounds_SG203_AC5(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{{-1, "MIN"}, {0, ""}, {MaxDeposit, ""}, {MaxDeposit + 1, "MAX"}}
	for _, c := range cases {
		v, err := ValidateDeposit(c.n)
		if c.want == "" {
			if err != nil || v != money.Vnd(c.n) {
				t.Errorf("%d: got %v err=%v", c.n, v, err)
			}
			continue
		}
		if got := codes(err); !reflect.DeepEqual(got, []FieldError{{"deposit", c.want}}) {
			t.Errorf("%d: errors = %v, want %s", c.n, got, c.want)
		}
	}
}

func TestMaskIDNumber_SG203_AC4(t *testing.T) {
	cases := map[string]string{
		"": "", "A": "*****", "ABCD": "*****", "ABCDE": "*****", "ABCDEFG": "*****", "ABCDEFGH": "*****FGH", "079123456789": "*****789",
	}
	for in, want := range cases {
		if got := MaskIDNumber(in); got != want {
			t.Errorf("MaskIDNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStatus_SG203_AC1(t *testing.T) {
	for _, s := range []Status{StatusActive, StatusCheckedOut} {
		if got, err := ParseStatus(string(s)); err != nil || got != s {
			t.Errorf("%s: %v %v", s, got, err)
		}
	}
	for _, s := range []string{"", "active", marker} {
		_, err := ParseStatus(s)
		if err == nil || strings.Contains(err.Error(), marker) {
			t.Errorf("%q must fail closed without echoing input: %v", s, err)
		}
	}
}
