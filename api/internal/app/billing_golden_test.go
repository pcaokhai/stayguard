package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

// Read at run time through code only; the file is a contract artifact.
const goldenPath = "../../../contracts/pricing/golden-cases.json"

// These mirror the constants in contracts/pricing/generate_vectors.py: the golden file carries only
// flat prices, so the windows and the plan version are filled in here (as in the SG-101 test).
const (
	goldenPlanVersion = 1
	overnightStartH   = 21
	overnightEndH     = 12
	dailyStartH       = 14
	dailyEndH         = 12
)

type goldenBill struct {
	Timezone     string `json:"timezone"`
	GraceMinutes int    `json:"graceMinutes"`
	RatePlans    map[string]struct {
		FirstHour, ExtraHour, Overnight, Daily int64
	} `json:"ratePlans"`
	Cases []struct {
		ID         string `json:"id"`
		RoomType   string `json:"roomType"`
		RentalType string `json:"rentalType"`
		CheckIn    string `json:"checkIn"`
		CheckOut   string `json:"checkOut"`
	} `json:"cases"`
	BillCases []struct {
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
	} `json:"billCases"`
}

func loadGoldenBill(t *testing.T) goldenBill {
	t.Helper()
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenBill
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func (g goldenBill) plan(t *testing.T, name string) pricing.RatePlan {
	t.Helper()
	p, ok := g.RatePlans[name]
	if !ok {
		t.Fatalf("golden file has no plan %q", name)
	}
	return pricing.RatePlan{
		Version: goldenPlanVersion, Currency: pricing.CurrencyVND, GraceMinutes: g.GraceMinutes,
		Hourly:    pricing.Hourly{FirstHour: money.Vnd(p.FirstHour), ExtraHour: money.Vnd(p.ExtraHour)},
		Overnight: pricing.Window{Price: money.Vnd(p.Overnight), Start: pricing.Clock{Hour: overnightStartH}, End: pricing.Clock{Hour: overnightEndH}},
		Daily:     pricing.Window{Price: money.Vnd(p.Daily), Start: pricing.Clock{Hour: dailyStartH}, End: pricing.Clock{Hour: dailyEndH}},
	}
}

func TestInvoiceBillCases_SG205_AC5(t *testing.T) {
	g := loadGoldenBill(t)
	if g.Timezone != zoneName || len(g.BillCases) == 0 {
		t.Fatalf("golden file: zone %q, %d bill cases", g.Timezone, len(g.BillCases))
	}
	refunds := 0
	for _, bc := range g.BillCases {
		var ref int
		found := false
		for i, c := range g.Cases {
			if c.ID == bc.CaseRef {
				ref, found = i, true
			}
		}
		if !found {
			t.Fatalf("%s: unknown caseRef %q", bc.ID, bc.CaseRef)
		}
		c := g.Cases[ref]
		in, err1 := time.Parse(time.RFC3339, c.CheckIn)
		out, err2 := time.Parse(time.RFC3339, c.CheckOut)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: %v %v", bc.ID, err1, err2)
		}
		e := newBillEnv(t)
		rec := StayRecord{RentalType: c.RentalType, CheckInAt: in, Deposit: bc.Deposit, RatePlanSnapshot: g.plan(t, c.RoomType).Snapshot()}
		for _, x := range bc.Extras {
			rec.Extras = append(rec.Extras, ExtraRecord{ServiceCode: x.ServiceCode, Quantity: x.Quantity, UnitAmount: x.UnitAmount})
		}
		e.seedBillStay(t, rec)
		e.clock.now = out
		v, _, err := e.checkout("st1", callID(1))
		if err != nil {
			t.Fatalf("%s: %v", bc.ID, err)
		}
		w, q := bc.Expected, v.Quote
		if q.StayAmount != w.StayTotal || q.ExtrasAmount != w.ExtrasTotal || q.Total != w.Total || q.BalanceDue != w.BalanceDue || q.RefundDue != w.RefundDue {
			t.Fatalf("%s: got %+v, want %+v", bc.ID, q, w)
		}
		if v.Quote.RefundDue > 0 {
			refunds++
		}
	}
	if refunds == 0 {
		t.Fatal("the golden file must exercise a refund")
	}
}
