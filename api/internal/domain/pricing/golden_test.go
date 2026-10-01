package pricing

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

const goldenPath = "../../../../contracts/pricing/golden-cases.json"

// These mirror the constants in contracts/pricing/generate_vectors.py; the golden file carries
// only flat prices, so the windows and version are filled in here.
const (
	goldenZone        = "Asia/Ho_Chi_Minh"
	goldenPlanVersion = 1
	overnightStartH   = 21
	overnightEndH     = 12
	dailyStartH       = 14
	dailyEndH         = 12
)

type goldenFile struct {
	Timezone     string                `json:"timezone"`
	GraceMinutes int                   `json:"graceMinutes"`
	Currency     string                `json:"currency"`
	RatePlans    map[string]goldenPlan `json:"ratePlans"`
	Cases        []goldenCase          `json:"cases"`
	ErrorCases   []goldenErrorCase     `json:"errorCases"`
	BillCases    []goldenBillCase      `json:"billCases"`
}

type goldenPlan struct {
	FirstHour, ExtraHour, Overnight, Daily int64
}

type goldenLine struct {
	Code       string `json:"code"`
	Quantity   int64  `json:"quantity"`
	UnitAmount int64  `json:"unitAmount"`
	Amount     int64  `json:"amount"`
}

type goldenCase struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RoomType   string `json:"roomType"`
	RentalType string `json:"rentalType"`
	CheckIn    string `json:"checkIn"`
	CheckOut   string `json:"checkOut"`
	Expected   struct {
		Total  int64        `json:"total"`
		Capped bool         `json:"capped"`
		Lines  []goldenLine `json:"lines"`
	} `json:"expected"`
}

type goldenErrorCase struct {
	ID            string `json:"id"`
	RoomType      string `json:"roomType"`
	RentalType    string `json:"rentalType"`
	CheckIn       string `json:"checkIn"`
	CheckOut      string `json:"checkOut"`
	ExpectedError string `json:"expectedError"`
}

type goldenBillCase struct {
	ID      string `json:"id"`
	CaseRef string `json:"caseRef"`
	Extras  []struct {
		ServiceCode string `json:"serviceCode"`
		Quantity    int64  `json:"quantity"`
		UnitAmount  int64  `json:"unitAmount"`
	} `json:"extras"`
	Deposit  int64 `json:"deposit"`
	Expected struct {
		StayTotal, ExtrasTotal, Total, BalanceDue, RefundDue int64
	} `json:"expected"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func (g goldenFile) plan(t *testing.T, name string) RatePlan {
	t.Helper()
	p, ok := g.RatePlans[name]
	if !ok {
		t.Fatalf("golden file has no plan %q", name)
	}
	return RatePlan{
		Version:      goldenPlanVersion,
		Currency:     CurrencyVND,
		GraceMinutes: g.GraceMinutes,
		Hourly:       Hourly{FirstHour: money.Vnd(p.FirstHour), ExtraHour: money.Vnd(p.ExtraHour)},
		Overnight:    Window{Price: money.Vnd(p.Overnight), Start: Clock{Hour: overnightStartH}, End: Clock{Hour: overnightEndH}},
		Daily:        Window{Price: money.Vnd(p.Daily), Start: Clock{Hour: dailyStartH}, End: Clock{Hour: dailyEndH}},
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func goldenZoneLoc(t *testing.T, g goldenFile) *time.Location {
	t.Helper()
	if g.Timezone != goldenZone {
		t.Fatalf("golden timezone = %q", g.Timezone)
	}
	loc, err := time.LoadLocation(g.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func priceCase(t *testing.T, g goldenFile, plan, rental, in, out string) (Quote, error) {
	t.Helper()
	rt, err := ParseRentalType(rental)
	if err != nil {
		t.Fatal(err)
	}
	return Price(g.plan(t, plan), rt, mustTime(t, in), mustTime(t, out), goldenZoneLoc(t, g))
}

func TestGoldenFileShape_SG101_AC1(t *testing.T) {
	g := loadGolden(t)
	if len(g.Cases) != 25 || len(g.ErrorCases) != 2 || len(g.BillCases) != 2 {
		t.Fatalf("shape = %d cases, %d error cases, %d bill cases", len(g.Cases), len(g.ErrorCases), len(g.BillCases))
	}
	for _, name := range []string{"STANDARD", "VIP"} {
		g.plan(t, name)
	}
	if g.Currency != CurrencyVND || g.GraceMinutes != 15 {
		t.Fatalf("currency %q grace %d", g.Currency, g.GraceMinutes)
	}
}

func TestPriceGolden_SG101_AC1(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.Cases {
		t.Run(c.ID, func(t *testing.T) {
			q, err := priceCase(t, g, c.RoomType, c.RentalType, c.CheckIn, c.CheckOut)
			if err != nil {
				t.Fatalf("%s: %v", c.ID, err)
			}
			if q.Total.Int64() != c.Expected.Total || q.Capped != c.Expected.Capped {
				t.Fatalf("%s: got total %d capped %v, want %d %v", c.ID, q.Total, q.Capped, c.Expected.Total, c.Expected.Capped)
			}
			if len(q.Lines) != len(c.Expected.Lines) {
				t.Fatalf("%s: got lines %+v, want %+v", c.ID, q.Lines, c.Expected.Lines)
			}
			for i, w := range c.Expected.Lines {
				l := q.Lines[i]
				if l.Code != w.Code || l.Quantity != w.Quantity || l.UnitAmount.Int64() != w.UnitAmount || l.Amount.Int64() != w.Amount {
					t.Fatalf("%s line %d: got %+v, want %+v", c.ID, i, l, w)
				}
			}
		})
	}
}

func TestPriceGoldenErrors_SG101_AC1(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.ErrorCases {
		t.Run(c.ID, func(t *testing.T) {
			if c.ExpectedError != "PRICING_INVALID_INTERVAL" {
				t.Fatalf("unmapped expected error %q", c.ExpectedError)
			}
			_, err := priceCase(t, g, c.RoomType, c.RentalType, c.CheckIn, c.CheckOut)
			if !errors.Is(err, ErrInvalidInterval) {
				t.Fatalf("%s: err = %v", c.ID, err)
			}
		})
	}
}

func TestBillGolden_SG101_AC5(t *testing.T) {
	g := loadGolden(t)
	byID := map[string]goldenCase{}
	for _, c := range g.Cases {
		byID[c.ID] = c
	}
	for _, b := range g.BillCases {
		t.Run(b.ID, func(t *testing.T) {
			ref, ok := byID[b.CaseRef]
			if !ok {
				t.Fatalf("%s: unknown caseRef %q", b.ID, b.CaseRef)
			}
			q, err := priceCase(t, g, ref.RoomType, ref.RentalType, ref.CheckIn, ref.CheckOut)
			if err != nil {
				t.Fatal(err)
			}
			extras := make([]Extra, 0, len(b.Extras))
			for _, e := range b.Extras {
				extras = append(extras, Extra{ServiceCode: e.ServiceCode, Quantity: e.Quantity, UnitAmount: money.Vnd(e.UnitAmount)})
			}
			got, err := Assemble(q, extras, money.Vnd(b.Deposit))
			if err != nil {
				t.Fatal(err)
			}
			want := Bill{
				StayTotal: money.Vnd(b.Expected.StayTotal), ExtrasTotal: money.Vnd(b.Expected.ExtrasTotal),
				Total: money.Vnd(b.Expected.Total), BalanceDue: money.Vnd(b.Expected.BalanceDue), RefundDue: money.Vnd(b.Expected.RefundDue),
			}
			if got != want {
				t.Fatalf("%s: got %+v, want %+v", b.ID, got, want)
			}
		})
	}
}
