// Package finance holds the pure money rules of payroll, expenses and the income and cost report (SG-1104, SG-1202,
// SG-1203): what a person earns, how months are named, and what an expense must say. All money is whole VND.
package finance

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// Pay types of a staff contract.
const (
	Monthly  = "MONTHLY"
	PerShift = "PER_SHIFT"
	Hourly   = "HOURLY"
)

// HoursPerShift is how many hours an hourly person is paid for one rostered shift.
const HoursPerShift = 8

// Expense sources and categories. PAYROLL, MAINTENANCE and STOCK lines are written by the system and never edited by hand.
const (
	Manual         = "MANUAL"
	Recurring      = "RECURRING"
	Payroll        = "PAYROLL"
	MaintenanceSrc = "MAINTENANCE"
	Stock          = "STOCK"

	maxNoteRunes = 300
	// MaxMonthsInReport bounds a report range.
	MaxMonthsInReport = 36
	// MaxExpense guards an amount against a typo and keeps sums far from overflow.
	MaxExpense int64 = 100_000_000_000
)

var categories = map[string]bool{"STAFF_PAY": true, "RENT": true, "ELECTRICITY": true, "WATER": true, "LAUNDRY": true, "MAINTENANCE": true,
	"SUPPLIES": true, "COST_OF_GOODS": true, "TAX_FEES": true, "INTERNET_TV": true, "PAYMENT_FEES": true, "OTHER": true}

// FieldError carries a field path and a code, never a value.
type FieldError struct{ Path, Code string }

// Contract is the pay terms of a person; Allowance is a fixed amount paid in full every month.
type Contract struct {
	PayType         string
	Rate, Allowance int64
	StandardShifts  int
}

// Work is what a person did in a month: rostered shifts they worked and rostered shifts that paid leave covered.
type Work struct{ Shifts, PaidLeaveShifts int }

func (w Work) credited() int64 { return int64(w.Shifts + w.PaidLeaveShifts) }

// Earned is the pay for the work, before allowance, bonus and deduction. MONTHLY pays the rate in full from the standard
// number of shifts and a share of it below that (never more than the rate); PER_SHIFT and HOURLY pay for every credited
// shift. A share is rounded to the nearest whole VND, half up.
func Earned(c Contract, w Work) (int64, error) {
	n := w.credited()
	switch c.PayType {
	case Monthly:
		if c.StandardShifts <= 0 || n >= int64(c.StandardShifts) {
			return c.Rate, nil
		}
		std := int64(c.StandardShifts)
		num, err := money.Mul(money.Vnd(c.Rate), n)
		if err != nil {
			return 0, err
		}
		return (num.Int64()*2 + std) / (2 * std), nil
	case PerShift:
		v, err := money.Mul(money.Vnd(c.Rate), n)
		return v.Int64(), err
	case Hourly:
		v, err := money.Mul(money.Vnd(c.Rate), n*HoursPerShift)
		return v.Int64(), err
	}
	return 0, fmt.Errorf("finance: unknown pay type %q", c.PayType)
}

// MaxDeduction is the gross pay: a deduction cannot take the net below zero.
func MaxDeduction(earned, allowance, bonus int64) int64 { return earned + allowance + bonus }

// Net is earned pay plus allowance and bonus, minus the deduction, never below zero.
func Net(earned, allowance, bonus, deduction int64) int64 {
	if n := MaxDeduction(earned, allowance, bonus) - deduction; n > 0 {
		return n
	}
	return 0
}

// IsAutomatic says whether a source is written by the system.
func IsAutomatic(source string) bool {
	return source == Payroll || source == MaintenanceSrc || source == Stock
}

func ValidCategory(c string) bool { return categories[c] }

// MonthDays parses YYYY-MM into its first and last day (midnight UTC).
func MonthDays(month string) (time.Time, time.Time, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil || len(month) != 7 {
		return time.Time{}, time.Time{}, errors.New("finance: month must be YYYY-MM")
	}
	return t, t.AddDate(0, 1, -1), nil
}

func MonthOf(t time.Time) string { return t.Format("2006-01") }

func NextMonth(m string) string {
	t, _ := time.Parse("2006-01", m)
	return MonthOf(t.AddDate(0, 1, 0))
}

func PrevMonth(m string) string {
	t, _ := time.Parse("2006-01", m)
	return MonthOf(t.AddDate(0, -1, 0))
}

// MonthsBetween lists the months from from to to, both included, at most MaxMonthsInReport.
func MonthsBetween(from, to string) ([]string, error) {
	if _, _, err := MonthDays(from); err != nil {
		return nil, err
	}
	if _, _, err := MonthDays(to); err != nil {
		return nil, err
	}
	var out []string
	for m := from; m <= to; m = NextMonth(m) {
		if len(out) == MaxMonthsInReport {
			return nil, errors.New("finance: range too long")
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, errors.New("finance: to is before from")
	}
	return out, nil
}

// MarginPct is profit as a percentage of revenue with one decimal; zero without revenue.
func MarginPct(revenue, expenses int64) float64 {
	if revenue <= 0 {
		return 0
	}
	return roundTenth(float64(revenue-expenses) * 100 / float64(revenue))
}

// OccupancyPct is occupied room-days as a percentage of available room-days, capped at 100.
func OccupancyPct(occupied, available int64) float64 {
	if available <= 0 {
		return 0
	}
	return roundTenth(min(float64(occupied)*100/float64(available), 100))
}

func roundTenth(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*10+0.5)) / 10
	}
	return float64(int64(v*10+0.5)) / 10
}

// CheckExpense lists what is wrong with an expense: a known category, an amount in range, a month, a payment day inside
// that month, and a short note. paidOn is empty or YYYY-MM-DD.
func CheckExpense(category string, amount int64, month, paidOn, note string) []FieldError {
	var errs []FieldError
	if !categories[category] {
		errs = append(errs, FieldError{"category", "PATTERN"})
	}
	if amount < 0 || amount > MaxExpense {
		errs = append(errs, FieldError{"amount", "MAX"})
	}
	first, last, err := MonthDays(month)
	if err != nil {
		errs = append(errs, FieldError{"month", "PATTERN"})
	}
	if paidOn != "" {
		d, perr := time.Parse(time.DateOnly, paidOn)
		if perr != nil || (err == nil && (d.Before(first) || d.After(last))) {
			errs = append(errs, FieldError{"paidOn", "PATTERN"})
		}
	}
	if utf8.RuneCountInString(strings.TrimSpace(note)) > maxNoteRunes {
		errs = append(errs, FieldError{"note", "TOO_LONG"})
	}
	return errs
}
