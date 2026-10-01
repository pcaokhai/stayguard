package pricing

import (
	"errors"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

var (
	ErrInvalidInterval   = errors.New("pricing: check-out must be after check-in")
	ErrUnknownRentalType = errors.New("pricing: unknown rental type")
)

type RentalType string

const (
	RentalHourly    RentalType = "HOURLY"
	RentalOvernight RentalType = "OVERNIGHT"
	RentalDaily     RentalType = "DAILY"
)

// ParseRentalType fails closed: anything but the three known values is an error.
func ParseRentalType(s string) (RentalType, error) {
	switch r := RentalType(s); r {
	case RentalHourly, RentalOvernight, RentalDaily:
		return r, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownRentalType, s)
}

const (
	CodeFirstHour       = "FIRST_HOUR"
	CodeExtraHour       = "EXTRA_HOUR"
	CodeOvernight       = "OVERNIGHT"
	CodeEarlyCheckinHr  = "EARLY_CHECKIN_HOUR"
	CodeLateCheckoutHr  = "LATE_CHECKOUT_HOUR"
	CodeDaily           = "DAILY"
	CodeEarlyCheckinFee = "EARLY_CHECKIN_FEE"
	CodeLateCheckoutFee = "LATE_CHECKOUT_FEE"
)

type Line struct {
	Code       string
	Quantity   int64
	UnitAmount money.Vnd
	Amount     money.Vnd
}

type Quote struct {
	Total  money.Vnd
	Capped bool
	Lines  []Line
}

func newLine(code string, qty int64, unit money.Vnd) (Line, error) {
	amount, err := money.Mul(unit, qty)
	if err != nil {
		return Line{}, fmt.Errorf("line %s: %w", code, err)
	}
	return Line{Code: code, Quantity: qty, UnitAmount: unit, Amount: amount}, nil
}

// optionalLine appends a line only when it has a quantity.
func optionalLine(lines []Line, code string, qty int64, unit money.Vnd) ([]Line, error) {
	if qty == 0 {
		return lines, nil
	}
	l, err := newLine(code, qty, unit)
	if err != nil {
		return nil, err
	}
	return append(lines, l), nil
}

// Price is a pure function of its arguments (docs/02 §7.7). Instants are compared as instants;
// loc decides which wall-clock day and window they fall in.
func Price(plan RatePlan, rental RentalType, checkIn, checkOut time.Time, loc *time.Location) (Quote, error) {
	if !checkOut.After(checkIn) {
		return Quote{}, ErrInvalidInterval
	}
	in, out := checkIn.In(loc), checkOut.In(loc)
	var lines []Line
	var err error
	switch rental {
	case RentalHourly:
		lines, err = hourlyLines(plan, in, out)
	case RentalOvernight:
		lines, err = overnightLines(plan, in, out, loc)
	case RentalDaily:
		lines, err = dailyLines(plan, in, out, loc)
	default:
		return Quote{}, fmt.Errorf("%w: %q", ErrUnknownRentalType, string(rental))
	}
	if err != nil {
		return Quote{}, err
	}
	return finish(plan, rental, lines)
}

// finish sums the lines and applies the daily-price cap to hourly and overnight stays only.
func finish(plan RatePlan, rental RentalType, lines []Line) (Quote, error) {
	amounts := make([]money.Vnd, len(lines))
	for i, l := range lines {
		amounts[i] = l.Amount
	}
	total, err := money.Sum(amounts...)
	if err != nil {
		return Quote{}, fmt.Errorf("quote total: %w", err)
	}
	capped := rental != RentalDaily && total > plan.Daily.Price
	if capped {
		total = plan.Daily.Price
	}
	return Quote{Total: total, Capped: capped, Lines: lines}, nil
}

func hourlyLines(plan RatePlan, in, out time.Time) ([]Line, error) {
	first, err := newLine(CodeFirstHour, 1, plan.Hourly.FirstHour)
	if err != nil {
		return nil, err
	}
	lines := []Line{first}
	mins := minutesBetween(in, out)
	if mins <= minutesPerHour {
		return lines, nil
	}
	return optionalLine(lines, CodeExtraHour, blocks(mins-minutesPerHour, plan.GraceMinutes), plan.Hourly.ExtraHour)
}

// overnightDay is the day the night starts on: a check-in before the window end belongs to the
// previous night, so a 01:00 arrival is priced as the night that began yesterday.
func overnightDay(plan RatePlan, in time.Time) int {
	tod := minuteOfDay(in)
	if tod >= plan.Overnight.Start.minuteOfDay() {
		return 0
	}
	if tod < plan.Overnight.End.minuteOfDay() {
		return -1
	}
	return 0
}

func overnightLines(plan RatePlan, in, out time.Time, loc *time.Location) ([]Line, error) {
	night, err := newLine(CodeOvernight, 1, plan.Overnight.Price)
	if err != nil {
		return nil, err
	}
	off := overnightDay(plan, in)
	start := wallAt(loc, in, off, plan.Overnight.Start)
	end := wallAt(loc, in, off+1, plan.Overnight.End)
	early := blocks(minutesBetween(in, start), plan.GraceMinutes)
	late := blocks(minutesBetween(end, out), plan.GraceMinutes)
	lines, err := optionalLine([]Line{night}, CodeEarlyCheckinHr, early, plan.Hourly.ExtraHour)
	if err != nil {
		return nil, err
	}
	return optionalLine(lines, CodeLateCheckoutHr, late, plan.Hourly.ExtraHour)
}

// cappedFee is blocks of extra hours, never more than one daily price.
func cappedFee(plan RatePlan, minutes int64) (money.Vnd, error) {
	fee, err := money.Mul(plan.Hourly.ExtraHour, blocks(minutes, plan.GraceMinutes))
	if err != nil {
		return 0, err
	}
	return min(fee, plan.Daily.Price), nil
}

// billingDays counts check-out noons passed: i is the first day offset whose window end is not before out.
func billingDays(plan RatePlan, in, out time.Time, loc *time.Location) int {
	i := 1
	for out.After(wallAt(loc, in, i, plan.Daily.End)) {
		i++
	}
	return i
}

func dailyLines(plan RatePlan, in, out time.Time, loc *time.Location) ([]Line, error) {
	early, err := cappedFee(plan, minutesBetween(in, wallAt(loc, in, 0, plan.Daily.Start)))
	if err != nil {
		return nil, err
	}
	i := billingDays(plan, in, out, loc)
	days, late := int64(1), money.Vnd(0)
	if i > 1 {
		days = int64(i - 1)
		if late, err = cappedFee(plan, minutesBetween(wallAt(loc, in, i-1, plan.Daily.End), out)); err != nil {
			return nil, err
		}
		if late >= plan.Daily.Price {
			days, late = days+1, 0
		}
	}
	lines, err := optionalLine(nil, CodeDaily, days, plan.Daily.Price)
	if err != nil {
		return nil, err
	}
	// Fees are a single line of quantity 1 whose unit is the fee itself.
	if early > 0 {
		lines = append(lines, Line{Code: CodeEarlyCheckinFee, Quantity: 1, UnitAmount: early, Amount: early})
	}
	if late > 0 {
		lines = append(lines, Line{Code: CodeLateCheckoutFee, Quantity: 1, UnitAmount: late, Amount: late})
	}
	return lines, nil
}
