package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/payment"
	"github.com/pcaokhai/stayguard/api/internal/domain/pricing"
)

var guesthouseCodePattern = regexp.MustCompile(`^[a-z0-9-]{3,16}$`)

// importFile is the JSON an installer fills with a customer's real data (kept out of git, deleted after import).
// Buildings, unit types and services follow contracts/fixtures/demo-tenant-seed.json.
type importFile struct {
	GuesthouseCode string `json:"guesthouseCode"`
	Name           string `json:"name"`
	Timezone       string `json:"timezone"`
	DefaultLocale  string `json:"defaultLocale"`
	Property       struct {
		Name    string  `json:"name"`
		Address *string `json:"address"`
		Phone   *string `json:"phone"`
	} `json:"property"`
	// BankAccount is added PENDING and becomes the QR default when the installer connects SePay for it.
	BankAccount *struct {
		BankBin     string `json:"bankBin"`
		AccountNo   string `json:"accountNo"`
		AccountName string `json:"accountName"`
	} `json:"bankAccount"`
	Buildings []struct {
		Code          string      `json:"code"`
		Name          string      `json:"name"`
		Floors        int         `json:"floors"`
		RoomsPerFloor roomsPerRow `json:"roomsPerFloor"`
		// FloorTypes names the unit type of each floor; missing means the first unit type.
		FloorTypes []string `json:"floorTypes"`
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
	Owner struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"owner"`
	Staff []struct {
		Name           string            `json:"name"`
		Username       *string           `json:"username"`
		Phone          *string           `json:"phone"`
		Position       string            `json:"position"`
		AppAccess      string            `json:"appAccess"`
		Contract       importContract    `json:"contract"`
		BuildingAccess map[string]string `json:"buildingAccess"`
	} `json:"staff"`
}

type importContract struct {
	PayType         string `json:"payType"`
	Rate            int64  `json:"rate"`
	FixedAllowance  int64  `json:"fixedAllowance"`
	StandardShifts  int    `json:"standardShifts"`
	StartDate       string `json:"startDate"`
	AnnualLeaveDays int    `json:"annualLeaveDays"`
}

// ImportedPin is a one-time PIN printed once to the installer.
type ImportedPin struct {
	Username  string
	Pin       string
	ExpiresAt time.Time
}

// ImportResult is what the CLI prints: ids and PINs, never secrets of the file.
type ImportResult struct {
	TenantID, GuesthouseCode, HookPath string
	Pins                               []ImportedPin
}

func (p ImportedPin) String() string   { return "ImportedPin{" + p.Username + " " + redacted + "}" }
func (p ImportedPin) GoString() string { return p.String() }

// Import creates a guesthouse with its buildings, rooms, rates, services, receiving account, owner and staff in one
// transaction. It prints one-time PINs (24 h) for everyone who can sign in.
func (i *Installer) Import(ctx context.Context, raw []byte) (ImportResult, error) {
	var f importFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return ImportResult{}, &ValidationError{"file", "not valid import JSON"}
	}
	if err := validateImport(f); err != nil {
		return ImportResult{}, err
	}
	tenantID := i.ids.New(tenantPrefix)
	hook, err := i.tokens.New()
	if err != nil {
		return ImportResult{}, fmt.Errorf("hook id: %w", err)
	}
	data, layoutCodes, err := i.buildLayout(tenantID, f)
	if err != nil {
		return ImportResult{}, err
	}
	now := i.clock.Now()
	res := ImportResult{TenantID: tenantID, GuesthouseCode: f.GuesthouseCode, HookPath: "/v1/webhooks/bank/" + hook}
	err = i.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error {
		locale, zone := f.DefaultLocale, f.Timezone
		if locale == "" {
			locale = "vi"
		}
		if zone == "" {
			zone = "Asia/Ho_Chi_Minh"
		}
		if err := i.setup.InsertTenant(ctx, tx, NewTenant{ID: tenantID, Name: f.Name, GuesthouseCode: f.GuesthouseCode, HookID: hook, Locale: locale, TimeZone: zone}); err != nil {
			return err
		}
		if err := i.layout.InsertDemoData(ctx, tx, data); err != nil {
			return err
		}
		pins, err := i.createPeople(ctx, tx, f, layoutCodes, now)
		if err != nil {
			return err
		}
		res.Pins = pins
		return i.importAudit(ctx, tx, tenantID)
	})
	return res, err
}

func validateImport(f importFile) error {
	switch {
	case !guesthouseCodePattern.MatchString(f.GuesthouseCode):
		return &ValidationError{"guesthouseCode", "3 to 16 of a-z 0-9 -"}
	case strings.TrimSpace(f.Name) == "":
		return &ValidationError{"name", "required"}
	case len(f.UnitTypes) == 0 || len(f.Buildings) == 0:
		return &ValidationError{"buildings", "at least one building and one unit type"}
	case !validUsername(f.Owner.Username) || strings.TrimSpace(f.Owner.Name) == "":
		return &ValidationError{"owner", "name and username required"}
	}
	if b := f.BankAccount; b != nil {
		if err := validateAccountInput(BankAccountInput{OwnerPin: "000000", BankBin: b.BankBin, AccountNo: b.AccountNo, AccountName: b.AccountName}); err != nil {
			return err
		}
	}
	for _, s := range f.Services {
		if s.Code == "" || s.Price < 0 || s.Stock < 0 {
			return &ValidationError{"services", "code, price and stock required and not negative"}
		}
	}
	return nil
}

// buildLayout turns the file into rows with ids; codes maps building code to id for staff access.
func (i *Installer) buildLayout(tenantID string, f importFile) (DemoData, map[string]string, error) {
	name := f.Property.Name
	if name == "" {
		name = f.Name
	}
	out := DemoData{SeededAt: i.clock.Now(), PropertyID: i.ids.New("pr"), PropertyName: name, PropertyAddress: f.Property.Address, PropertyPhone: f.Property.Phone}
	types := map[string]DemoUnitType{}
	for _, u := range f.UnitTypes {
		plan, err := pricing.ParseRatePlan(u.RatePlan)
		if err != nil {
			return DemoData{}, nil, &ValidationError{"unitTypes." + u.Code, "invalid rate plan"}
		}
		ut := DemoUnitType{ID: i.ids.New("ut"), Code: u.Code, Name: u.Name, RatePlan: plan.Snapshot(), Version: int32(plan.Version)} //nolint:gosec // parsed rate plan version
		types[u.Code] = ut
		out.UnitTypes = append(out.UnitTypes, ut)
	}
	for _, s := range f.Services {
		out.Services = append(out.Services, DemoService{i.ids.New("sv"), s.Code, s.Name, s.Price, s.Stock})
	}
	codes := map[string]string{}
	for _, b := range f.Buildings {
		bid := i.ids.New("bd")
		codes[b.Code] = bid
		out.Buildings = append(out.Buildings, DemoBuilding{bid, b.Code, b.Name})
		for fl := 0; fl < b.Floors; fl++ {
			fid := i.ids.New("fl")
			out.Floors = append(out.Floors, DemoFloor{fid, bid, int32(fl + 1)}) //nolint:gosec // a building has few floors
			typeCode := f.UnitTypes[0].Code
			if fl < len(b.FloorTypes) {
				typeCode = b.FloorTypes[fl]
			}
			ut, ok := types[typeCode]
			if !ok {
				return DemoData{}, nil, &ValidationError{"buildings." + b.Code + ".floorTypes", "unknown unit type"}
			}
			if len(b.RoomsPerFloor) == 0 {
				return DemoData{}, nil, &ValidationError{"buildings." + b.Code + ".roomsPerFloor", "required"}
			}
			for n := 1; n <= b.RoomsPerFloor.onFloor(fl); n++ {
				out.Rooms = append(out.Rooms, DemoRoom{i.ids.New("un"), bid, fid, ut.ID, fmt.Sprintf("%s%d%02d", b.Code, fl+1, n), "VACANT"})
			}
		}
	}
	if b := f.BankAccount; b != nil {
		acc, err := i.importAccount(tenantID, b.BankBin, b.AccountNo, b.AccountName)
		if err != nil {
			return DemoData{}, nil, err
		}
		out.BankAccounts = []NewBankAccount{acc}
	}
	return out, codes, nil
}

func (i *Installer) importAccount(tenantID, bin, no, name string) (NewBankAccount, error) {
	id := i.ids.New(bankIDPrefix)
	plain, err := json.Marshal(map[string]string{"bankBin": bin, "accountNo": no, "accountName": name})
	if err != nil {
		return NewBankAccount{}, err
	}
	enc, err := i.enc.Encrypt(tenantID, bankAccountFieldOf(id), plain)
	if err != nil {
		return NewBankAccount{}, fmt.Errorf("encrypt bank account: %w", err)
	}
	return NewBankAccount{ID: id, BankBin: bin, BankName: payment.BankName(bin), Masked: maskAccount(no), Name: name,
		Fingerprint: hex.EncodeToString(i.enc.Fingerprint(tenantID, bankAccountField, []byte(bin+"/"+no))),
		Enc:         enc, MakeDefaultWhenConnected: true}, nil
}

// createPeople stores the owner and staff with their building access and one-time PINs.
func (i *Installer) createPeople(ctx context.Context, tx Tx, f importFile, buildings map[string]string, now time.Time) ([]ImportedPin, error) {
	var pins []ImportedPin
	exp := now.Add(OneTimePinTTL)
	issue := func(userID, username string) error {
		pin, err := i.pins.New()
		if err != nil {
			return err
		}
		hash, err := i.hasher.Hash(pin)
		if err != nil {
			return err
		}
		if err = i.authDB.SetPin(ctx, tx, userID, hash, true, &exp, now); err != nil {
			return err
		}
		pins = append(pins, ImportedPin{Username: username, Pin: pin, ExpiresAt: exp})
		return nil
	}
	ownerID := i.ids.New(userPrefix)
	if err := i.staff.InsertOwner(ctx, tx, ownerID, f.Owner.Name, f.Owner.Username); err != nil {
		return nil, err
	}
	if err := issue(ownerID, f.Owner.Username); err != nil {
		return nil, err
	}
	for _, s := range f.Staff {
		k, err := importContractOf(s.Contract)
		if err != nil {
			return nil, err
		}
		in := StaffInput{Name: s.Name, Position: s.Position, AppAccess: s.AppAccess, Phone: s.Phone, Username: s.Username, Contract: k}
		if err = validateStaffInput(in); err != nil {
			return nil, err
		}
		id := i.ids.New(userPrefix)
		username := s.Username
		if s.AppAccess == accessNone {
			username = nil
		}
		if err = i.staff.InsertStaff(ctx, tx, StaffInsert{ID: id, Name: s.Name, Role: string(appAccessRole[s.AppAccess]), AppAccess: s.AppAccess,
			Username: username, Phone: s.Phone, Position: s.Position, Contract: k}); err != nil {
			return nil, err
		}
		if s.AppAccess == accessNone {
			continue
		}
		if err = issue(id, *username); err != nil {
			return nil, err
		}
		for code, lv := range s.BuildingAccess {
			bid, ok := buildings[code]
			level, perr := access.ParseLevel(lv)
			if !ok || perr != nil {
				return nil, &ValidationError{"staff.buildingAccess", "unknown building or level"}
			}
			if err = i.staff.SetBuildingLevel(ctx, tx, id, bid, level, now); err != nil {
				return nil, err
			}
		}
	}
	return pins, nil
}

func importContractOf(c importContract) (Contract, error) {
	d, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return Contract{}, &ValidationError{"staff.contract.startDate", "a date as YYYY-MM-DD"}
	}
	return Contract{PayType: c.PayType, Rate: moneyOf(c.Rate), FixedAllowance: moneyOf(c.FixedAllowance), StandardShifts: c.StandardShifts,
		StartDate: d, AnnualLeaveDays: c.AnnualLeaveDays}, nil
}
