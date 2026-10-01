package postgres

import (
	"os"
	"strings"
	"testing"
)

// TestOnlySettleQuerySetsPaid_A2 keeps CLAUDE.md §4.4 true: a transfer becomes PAID only through SettleTransfer.
func TestOnlySettleQuerySetsPaid_A2(t *testing.T) {
	files, err := os.ReadDir("queries")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		raw, err := os.ReadFile("queries/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(raw), "-- name: ")[1:] {
			name, _, _ := strings.Cut(block, " ")
			sql := strings.ToUpper(block)
			setsPaid := strings.Contains(sql, "SET STATUS = 'PAID'") && strings.Contains(sql, "APP.PAYMENTS")
			insertsPaid := strings.Contains(sql, "INSERT INTO APP.PAYMENTS") && strings.Contains(sql, "'PAID'")
			switch {
			case setsPaid && name != "SettleTransfer":
				t.Errorf("%s sets a payment to PAID", name)
			case insertsPaid && (name != "InsertCashPayment" || !strings.Contains(sql, "'CASH'")):
				t.Errorf("%s inserts a PAID payment that is not CASH", name)
			}
		}
	}
}
