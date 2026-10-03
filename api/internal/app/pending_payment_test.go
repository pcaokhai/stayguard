package app

import (
	"testing"
	"time"
)

// pendingPayment.remaining is the invoice total minus the deposit minus the bank money matched to it, never below zero, whether or
// not a payment record exists yet.
func TestNewPendingPayment_Remaining_FU(t *testing.T) {
	at := time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name                                string
		total, deposit, received, refundDue int64
		wantRemaining                       int64
	}{
		{"no payment, no deposit", 80_000, 0, 0, 0, 80_000},
		{"deposit below the total", 80_000, 30_000, 0, 0, 50_000},
		{"deposit equal to the total", 80_000, 80_000, 0, 0, 0},
		{"deposit above the total (refund due)", 80_000, 100_000, 0, 20_000, 0},
		{"partial transfer", 80_000, 0, 30_000, 0, 50_000},
		{"partial transfer on top of a deposit", 80_000, 30_000, 20_000, 0, 30_000},
		{"partial cash (a deposit is the cash taken before the bill)", 80_000, 50_000, 0, 0, 30_000},
		{"bank money above what is left", 80_000, 30_000, 70_000, 0, 0},
	} {
		p := NewPendingPayment("", c.total, c.deposit, c.received, c.refundDue, at)
		if p.Remaining != c.wantRemaining || p.Total != c.total || p.Deposit != c.deposit || p.Received != c.received || p.RefundDue != c.refundDue || !p.CreatedAt.Equal(at) {
			t.Errorf("%s: %+v, want remaining %d", c.name, p, c.wantRemaining)
		}
	}
}
