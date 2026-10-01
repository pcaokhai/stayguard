package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

// demoSeedJSON is a copy of contracts/fixtures/demo-tenant-seed.json (a test keeps them equal),
// because embedding cannot reach outside the module.
//
//go:embed demo-tenant-seed.json
var demoSeedJSON []byte

const (
	bankAccountField = "bank_account"
	vipFromFloor     = 3 // the seed names no room types; the top floor is VIP
	demoPhone        = "0900000000"
)

// DemoData is a trial tenant's starting rows with every id already generated.
type DemoData struct {
	PropertyID, PropertyName string
	BankAccountEnc           []byte
	Buildings                []DemoBuilding
	Floors                   []DemoFloor
	UnitTypes                []DemoUnitType
	Rooms                    []DemoRoom
	Services                 []DemoService
	Stays                    []NewStay
}

type DemoBuilding struct{ ID, Code, Name string }
type DemoFloor struct {
	ID, BuildingID string
	Level          int32
}
type DemoUnitType struct {
	ID, Code string
	Name     []byte
	RatePlan []byte
	Version  int32
}
type DemoRoom struct{ ID, BuildingID, FloorID, UnitTypeID, Code, Status string }
type DemoService struct {
	ID, Code string
	Name     []byte
	Price    int64
	Stock    int64
}

// DemoSeedRepo writes DemoData in the Tx's tenant.
type DemoSeedRepo interface {
	InsertDemoData(ctx context.Context, tx Tx, d DemoData) error
}

// DemoSeeder fills a new trial tenant from the embedded seed.
type DemoSeeder struct {
	repo DemoSeedRepo
	enc  Encryptor
	ids  IDGenerator
}

func NewDemoSeeder(repo DemoSeedRepo, enc Encryptor, ids IDGenerator) *DemoSeeder {
	return &DemoSeeder{repo, enc, ids}
}

type seedFile struct {
	Tenant struct {
		BankAccount json.RawMessage `json:"bankAccount"`
	} `json:"tenant"`
	Buildings []struct {
		Code          string      `json:"code"`
		Name          string      `json:"name"`
		Floors        int         `json:"floors"`
		RoomsPerFloor roomsPerRow `json:"roomsPerFloor"`
	} `json:"buildings"`
	UnitTypes []struct {
		Code     string          `json:"code"`
		Name     json.RawMessage `json:"name"`
		RatePlan json.RawMessage `json:"ratePlan"`
	} `json:"unitTypes"`
	Services []struct {
		Code  string          `json:"code"`
		Name  json.RawMessage `json:"name"`
		Price int64           `json:"price"`
		Stock int64           `json:"stock"`
	} `json:"services"`
	SampleOccupancy struct {
		OccupiedHourly, OccupiedDaily, OverdueDaily, ToClean, Maintenance []string
	} `json:"sampleOccupancy"`
}

// roomsPerRow is a number (same on every floor) or one number per floor.
type roomsPerRow []int

func (r *roomsPerRow) UnmarshalJSON(b []byte) error {
	var one int
	if json.Unmarshal(b, &one) == nil {
		*r = roomsPerRow{one}
		return nil
	}
	return json.Unmarshal(b, (*[]int)(r))
}

func (r roomsPerRow) onFloor(i int) int {
	if len(r) == 1 {
		return r[0]
	}
	return r[i]
}

// stayAge is how long ago a sample stay checked in, by the server clock minus a fixed offset.
var (
	hourlyAges  = []time.Duration{25 * time.Minute, 50 * time.Minute, 85 * time.Minute, 130 * time.Minute, 20 * time.Minute, 70 * time.Minute, 100 * time.Minute}
	dailyAges   = []time.Duration{5 * time.Hour, 9 * time.Hour, 18 * time.Hour}
	overdueAge  = 50 * time.Hour
	sampleDepos = int64(100000)
)

// Seed inserts the demo tenant's rooms, prices, services, bank account and sample stays.
func (d *DemoSeeder) Seed(ctx context.Context, tx Tx, now time.Time) error {
	data, err := d.build(tx.TenantID(), now)
	if err != nil {
		return err
	}
	return d.repo.InsertDemoData(ctx, tx, data)
}

func (d *DemoSeeder) build(tenantID string, now time.Time) (DemoData, error) {
	var f seedFile
	if err := json.Unmarshal(demoSeedJSON, &f); err != nil {
		return DemoData{}, fmt.Errorf("demo seed: %w", err)
	}
	enc, err := d.enc.Encrypt(tenantID, bankAccountField, f.Tenant.BankAccount)
	if err != nil {
		return DemoData{}, fmt.Errorf("demo seed bank account: %w", err)
	}
	out := DemoData{PropertyID: d.ids.New("pr"), PropertyName: trialTenantName, BankAccountEnc: enc}
	plans := map[string]DemoUnitType{}
	for _, u := range f.UnitTypes {
		plan, err := pricing.ParseRatePlan(u.RatePlan)
		if err != nil {
			return DemoData{}, fmt.Errorf("demo seed unit type %s: %w", u.Code, err)
		}
		ut := DemoUnitType{ID: d.ids.New("ut"), Code: u.Code, Name: u.Name, RatePlan: plan.Snapshot(), Version: int32(plan.Version)} //nolint:gosec // embedded fixture, version 1
		plans[u.Code] = ut
		out.UnitTypes = append(out.UnitTypes, ut)
	}
	for _, s := range f.Services {
		out.Services = append(out.Services, DemoService{d.ids.New("sv"), s.Code, s.Name, s.Price, s.Stock})
	}
	for _, b := range f.Buildings {
		bid := d.ids.New("bd")
		out.Buildings = append(out.Buildings, DemoBuilding{bid, b.Code, b.Name})
		for fl := 0; fl < b.Floors; fl++ {
			fid := d.ids.New("fl")
			out.Floors = append(out.Floors, DemoFloor{fid, bid, int32(fl + 1)})
			typ := "STANDARD"
			if fl+1 >= vipFromFloor {
				typ = "VIP"
			}
			for n := 1; n <= b.RoomsPerFloor.onFloor(fl); n++ {
				code := fmt.Sprintf("%s%d%02d", b.Code, fl+1, n)
				out.Rooms = append(out.Rooms, DemoRoom{d.ids.New("un"), bid, fid, plans[typ].ID, code, "VACANT"})
			}
		}
	}
	return d.occupy(out, f, plans, now)
}

// occupy sets the sample statuses and adds one ACTIVE stay per occupied room.
func (d *DemoSeeder) occupy(out DemoData, f seedFile, plans map[string]DemoUnitType, now time.Time) (DemoData, error) {
	byCode := map[string]int{}
	for i, r := range out.Rooms {
		byCode[r.Code] = i
	}
	typeByID := map[string]DemoUnitType{}
	for _, u := range plans {
		typeByID[u.ID] = u
	}
	set := func(codes []string, status string, rental string, ages []time.Duration) error {
		for i, c := range codes {
			ix, ok := byCode[c]
			if !ok {
				return fmt.Errorf("demo seed: unknown room %s", c)
			}
			out.Rooms[ix].Status = status
			if rental == "" {
				continue
			}
			out.Stays = append(out.Stays, NewStay{
				ID: d.ids.New("st"), RoomID: out.Rooms[ix].ID, RentalType: rental,
				GuestName: fmt.Sprintf("Khách %s", c), GuestPhone: demoPhone, Deposit: sampleDepos,
				CheckInAt:        now.Add(-ages[i%len(ages)]),
				RatePlanSnapshot: typeByID[out.Rooms[ix].UnitTypeID].RatePlan, RatePlanSchema: rateSnapshotSchema,
			})
		}
		return nil
	}
	o := f.SampleOccupancy
	for _, step := range []struct {
		codes          []string
		status, rental string
		ages           []time.Duration
	}{
		{o.OccupiedHourly, "OCCUPIED", "HOURLY", hourlyAges},
		{o.OccupiedDaily, "OCCUPIED", "DAILY", dailyAges},
		{o.OverdueDaily, "OCCUPIED", "DAILY", []time.Duration{overdueAge}},
		{o.ToClean, "TO_CLEAN", "", nil},
		{o.Maintenance, "MAINTENANCE", "", nil},
	} {
		if err := set(step.codes, step.status, step.rental, step.ages); err != nil {
			return DemoData{}, err
		}
	}
	return out, nil
}
