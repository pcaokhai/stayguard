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
		{"valid all", "  Nguyen An ", " +84 (90) 123-4567 ", " AB12345 ", nil, Guest{"Nguyen An", "+84 (90) 123-4567", "AB12345"}},
		{"valid no id", "A", "123456", "", nil, Guest{"A", "123456", ""}},
		{"id only spaces is none", "A", "123456", "   ", nil, Guest{"A", "123456", ""}},
		{"name 120 runes", strings.Repeat("a", 120), "123456", "", nil, Guest{strings.Repeat("a", 120), "123456", ""}},
		{"name 120 multibyte", strings.Repeat("ễ", 120), "123456", "", nil, Guest{strings.Repeat("ễ", 120), "123456", ""}},
		{"name 121 runes", strings.Repeat("ễ", 121), "123456", "", []FieldError{{"guestName", "TOO_LONG"}}, Guest{}},
		{"name empty", "", "123456", "", []FieldError{{"guestName", "REQUIRED"}}, Guest{}},
		{"name spaces only", "   ", "123456", "", []FieldError{{"guestName", "REQUIRED"}}, Guest{}},
		{"name control", "a\x07b", "123456", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"name newline inside", "a\nb", "123456", "", []FieldError{{"guestName", "PATTERN"}}, Guest{}},
		{"phone 5 digits", "A", "12345", "", []FieldError{{"guestPhone", "TOO_SHORT"}}, Guest{}},
		{"phone 6", "A", "123456", "", nil, Guest{"A", "123456", ""}},
		{"phone 20", "A", strings.Repeat("1", 20), "", nil, Guest{"A", strings.Repeat("1", 20), ""}},
		{"phone 21", "A", strings.Repeat("1", 21), "", []FieldError{{"guestPhone", "TOO_LONG"}}, Guest{}},
		{"phone empty", "A", "", "", []FieldError{{"guestPhone", "REQUIRED"}}, Guest{}},
		{"phone 5 digits with separators", "A", "1-2.3 45 ()", "", []FieldError{{"guestPhone", "TOO_SHORT"}}, Guest{}},
		{"phone plus middle", "A", "123+456789", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"phone letters", "A", "12345a6789", "", []FieldError{{"guestPhone", "PATTERN"}}, Guest{}},
		{"id 4", "A", "123456", "AB12", []FieldError{{"idNumber", "TOO_SHORT"}}, Guest{}},
		{"id 5", "A", "123456", "AB123", nil, Guest{"A", "123456", "AB123"}},
		{"id 20", "A", "123456", strings.Repeat("9", 20), nil, Guest{"A", "123456", strings.Repeat("9", 20)}},
		{"id 21", "A", "123456", strings.Repeat("9", 21), []FieldError{{"idNumber", "TOO_LONG"}}, Guest{}},
		{"id non ascii", "A", "123456", "ABCDÉ", []FieldError{{"idNumber", "PATTERN"}}, Guest{}},
		{"id symbols", "A", "123456", "AB-123", []FieldError{{"idNumber", "PATTERN"}}, Guest{}},
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
		"": "", "A": "*****", "ABCD": "*****", "ABCDE": "*****CDE", "079123456789": "*****789",
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
	for _, s := range []string{"", "active", "OVERDUE"} {
		if _, err := ParseStatus(s); err == nil {
			t.Errorf("%q must fail closed", s)
		}
	}
}
