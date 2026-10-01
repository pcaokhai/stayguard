package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// errCorruptStoredRentalType carries no value: the stored string could be tenant data.
var errCorruptStoredRentalType = errors.New("stay has an unknown stored rental type")

// StayDetail is the check-in and get-stay answer. It is also the stored idempotency response body,
// so it must round-trip through JSON and never holds the plain ID number. (StayView is the room
// map's compact active stay.)
type StayDetail struct {
	ID             string
	RoomID         string
	RoomCode       string
	RentalType     string
	Status         string
	CheckInAt      time.Time
	CheckOutAt     *time.Time
	GuestName      string
	GuestPhone     string
	IDNumberMasked *string
	Deposit        int64
	Extras         []ExtraView
	Quote          QuoteView
	PricingVersion int
}

type ExtraView struct {
	ServiceCode string
	Name        LocalizedName
	Quantity    int64
	UnitAmount  int64
	Amount      int64
}

type QuoteView struct {
	AsOf         time.Time
	StayAmount   int64
	ExtrasAmount int64
	Total        int64
	DepositPaid  int64
	BalanceDue   int64
	RefundDue    int64
	Capped       bool
	Lines        []LineView
}

type LineView struct {
	Code       string
	Quantity   int64
	UnitAmount int64
	Amount     int64
}

// detailFor prices rec as of asOf from its own snapshot and assembles the view. masked is the
// already-masked ID number (nil when none).
func detailFor(rec StayRecord, asOf time.Time, masked *string, loc *time.Location) (StayDetail, error) {
	plan, err := pricing.ParseRatePlan(rec.RatePlanSnapshot)
	if err != nil {
		return StayDetail{}, fmt.Errorf("stay rate plan snapshot: %w", err)
	}
	rental, err := pricing.ParseRentalType(rec.RentalType)
	if err != nil { // stored data, not client input: a distinct error so it ends as a logged 500, never a 422
		return StayDetail{}, errCorruptStoredRentalType
	}
	q, err := pricing.QuoteRunning(plan, rental, rec.CheckInAt, asOf, loc)
	if err != nil {
		return StayDetail{}, fmt.Errorf("stay quote: %w", err)
	}
	extras, views, err := extrasFor(rec.Extras)
	if err != nil {
		return StayDetail{}, err
	}
	deposit, err := money.NewVnd(rec.Deposit)
	if err != nil {
		return StayDetail{}, fmt.Errorf("stay deposit: %w", err)
	}
	bill, err := pricing.Assemble(q, extras, deposit)
	if err != nil {
		return StayDetail{}, fmt.Errorf("stay bill: %w", err)
	}
	return StayDetail{
		ID: rec.ID, RoomID: rec.RoomID, RoomCode: rec.RoomCode, RentalType: rec.RentalType, Status: rec.Status,
		CheckInAt: rec.CheckInAt.UTC(), CheckOutAt: utcPtr(rec.CheckOutAt), GuestName: rec.GuestName,
		GuestPhone: rec.GuestPhone, IDNumberMasked: masked, Deposit: rec.Deposit, Extras: views,
		Quote:          quoteViewOf(asOf, q, bill, rec.Deposit),
		PricingVersion: int(plan.Version),
	}, nil
}

func quoteViewOf(asOf time.Time, q pricing.Quote, bill pricing.Bill, deposit int64) QuoteView {
	return QuoteView{
		AsOf: asOf.UTC(), StayAmount: bill.StayTotal.Int64(), ExtrasAmount: bill.ExtrasTotal.Int64(),
		Total: bill.Total.Int64(), DepositPaid: deposit, BalanceDue: bill.BalanceDue.Int64(),
		RefundDue: bill.RefundDue.Int64(), Capped: q.Capped, Lines: lineViews(q.Lines),
	}
}

func extrasFor(rows []ExtraRecord) ([]pricing.Extra, []ExtraView, error) {
	extras := make([]pricing.Extra, len(rows))
	views := make([]ExtraView, len(rows))
	for i, r := range rows {
		unit, err := money.NewVnd(r.UnitAmount)
		if err != nil {
			return nil, nil, fmt.Errorf("stay extra %s: %w", r.ServiceCode, err)
		}
		amount, err := money.Mul(unit, r.Quantity)
		if err != nil {
			return nil, nil, fmt.Errorf("stay extra %s: %w", r.ServiceCode, err)
		}
		extras[i] = pricing.Extra{ServiceCode: r.ServiceCode, Quantity: r.Quantity, UnitAmount: unit}
		views[i] = ExtraView{r.ServiceCode, r.Name, r.Quantity, r.UnitAmount, amount.Int64()}
	}
	return extras, views, nil
}

func lineViews(lines []pricing.Line) []LineView {
	out := make([]LineView, len(lines))
	for i, l := range lines {
		out[i] = LineView{l.Code, l.Quantity, l.UnitAmount.Int64(), l.Amount.Int64()}
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func maskedID(plain string) *string {
	if plain == "" {
		return nil
	}
	m := stay.MaskIDNumber(plain)
	return &m
}
