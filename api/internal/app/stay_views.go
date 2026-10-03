package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

// errCorruptStoredRentalType carries no value: the stored string could be tenant data.
var errCorruptStoredRentalType = errors.New("stay has an unknown stored rental type")

// StayDetail is the check-in and get-stay answer. It is also the stored idempotency response body,
// so it must round-trip through JSON and never holds the plain ID number. (StayView is the room
// map's compact active stay.)
type StayDetail struct {
	ID             string            `json:"id"`
	RoomID         string            `json:"roomId"`
	RoomCode       string            `json:"roomCode"`
	RentalType     string            `json:"rentalType"`
	Status         string            `json:"status"`
	CheckInAt      time.Time         `json:"checkInAt"`
	CheckOutAt     *time.Time        `json:"checkOutAt,omitempty"`
	GuestName      string            `json:"guestName"`
	GuestPhone     string            `json:"guestPhone"`
	GuestID        GuestIDIndicators `json:"guestId"`
	Deposit        int64             `json:"deposit"`
	Extras         []ExtraView       `json:"extras"`
	Quote          QuoteView         `json:"quote"`
	PricingVersion int               `json:"pricingVersion"`
	// PendingPayment is set while the stay is checked out and its invoice is not paid.
	PendingPayment *PendingPayment `json:"pendingPayment,omitempty"`
}

type ExtraView struct {
	ServiceCode string        `json:"serviceCode"`
	Name        LocalizedName `json:"name"`
	Quantity    int64         `json:"quantity"`
	UnitAmount  int64         `json:"unitAmount"`
	Amount      int64         `json:"amount"`
}

type QuoteView struct {
	AsOf         time.Time  `json:"asOf"`
	StayAmount   int64      `json:"stayAmount"`
	ExtrasAmount int64      `json:"extrasAmount"`
	Total        int64      `json:"total"`
	DepositPaid  int64      `json:"depositPaid"`
	BalanceDue   int64      `json:"balanceDue"`
	RefundDue    int64      `json:"refundDue"`
	Capped       bool       `json:"capped"`
	Lines        []LineView `json:"lines"`
}

type LineView struct {
	Code       string `json:"code"`
	Quantity   int64  `json:"quantity"`
	UnitAmount int64  `json:"unitAmount"`
	Amount     int64  `json:"amount"`
}

// detailFor prices rec as of asOf from its own snapshot and assembles the view. Guest ID data is never in it,
// only the indicators.
func detailFor(rec StayRecord, asOf time.Time, loc *time.Location) (StayDetail, error) {
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
		GuestPhone: rec.GuestPhone, GuestID: rec.GuestID, Deposit: rec.Deposit, Extras: views,
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
