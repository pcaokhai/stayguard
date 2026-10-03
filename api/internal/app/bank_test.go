package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

type fakeBankRepo struct {
	accounts map[string]*BankAccountRow
	fps      map[string]bool
	order    []string
	prop     PropertyView
	patched  PropertyPatch
	secret   []byte
	hook     string
}

func newFakeBankRepo() *fakeBankRepo {
	return &fakeBankRepo{accounts: map[string]*BankAccountRow{}, fps: map[string]bool{},
		prop: PropertyView{ID: "pr1", GuesthouseCode: "casa", Name: "Casa", QRExpiryMinutes: 30, IDRetentionDays: 30, FrontDeskHistoryDays: 7}}
}

func (r *fakeBankRepo) Property(context.Context, Tx) (PropertyView, bool, error) {
	return r.prop, true, nil
}

func (r *fakeBankRepo) UpdateProperty(_ context.Context, _ Tx, _ string, p PropertyPatch) error {
	r.patched = p
	if p.QRExpiryMinutes != nil {
		r.prop.QRExpiryMinutes = *p.QRExpiryMinutes
	}
	return nil
}

func (r *fakeBankRepo) ListAccounts(context.Context, Tx) ([]BankAccountRow, error) {
	var out []BankAccountRow
	for _, id := range r.order {
		out = append(out, *r.accounts[id])
	}
	return out, nil
}

func (r *fakeBankRepo) Account(_ context.Context, _ Tx, id string) (BankAccountRow, bool, error) {
	a, ok := r.accounts[id]
	if !ok {
		return BankAccountRow{}, false, nil
	}
	return *a, true, nil
}

func (r *fakeBankRepo) InsertAccount(_ context.Context, _ Tx, a NewBankAccount) error {
	if r.fps[a.Fingerprint] {
		return ErrConflict
	}
	r.fps[a.Fingerprint] = true
	status := statusPending
	if a.Connected {
		status = statusConnected
	}
	r.accounts[a.ID] = &BankAccountRow{BankAccountView: BankAccountView{ID: a.ID, BankBin: a.BankBin, BankName: a.BankName, AccountNoMasked: a.Masked,
		AccountName: a.Name, IsDefault: a.IsDefault, SepayStatus: status}, MakeDefaultWhenConnected: a.MakeDefaultWhenConnected}
	r.order = append(r.order, a.ID)
	return nil
}

func (r *fakeBankRepo) SetDefault(_ context.Context, _ Tx, id string) (bool, error) {
	if r.accounts[id].SepayStatus != statusConnected {
		return false, nil
	}
	for _, a := range r.accounts {
		a.IsDefault = false
	}
	r.accounts[id].IsDefault = true
	return true, nil
}

func (r *fakeBankRepo) Connect(_ context.Context, _ Tx, id string) error {
	r.accounts[id].SepayStatus = statusConnected
	return nil
}

func (r *fakeBankRepo) DeleteAccount(_ context.Context, _ Tx, id string) (bool, error) {
	a, ok := r.accounts[id]
	if !ok || a.IsDefault {
		return false, nil
	}
	delete(r.accounts, id)
	return true, nil
}

func (r *fakeBankRepo) SepayState(context.Context, Tx) (SepayState, error) {
	s := SepayState{HasSecret: r.secret != nil, HookID: r.hook}
	for _, a := range r.accounts {
		if a.IsDefault && a.SepayStatus == statusConnected {
			s.DefaultConnected = true
		}
	}
	return s, nil
}

func (r *fakeBankRepo) SetHook(_ context.Context, _ Tx, h string) error { r.hook = h; return nil }

func (r *fakeBankRepo) SecretEnc(context.Context, Tx) ([]byte, error) { return r.secret, nil }

func (r *fakeBankRepo) AccountBlobs(context.Context, Tx) ([]AccountBlob, error) { return nil, nil }

func (r *fakeBankRepo) TouchWebhook(context.Context, Tx, string, time.Time) error { return nil }

func (r *fakeBankRepo) SetSignatureOK(context.Context, Tx, bool) error { return nil }

func (r *fakeBankRepo) SetSecret(_ context.Context, _ Tx, enc []byte) error {
	r.secret = enc
	return nil
}

type bankRig struct {
	b     *Bank
	repo  *fakeBankRepo
	audit *fakeAudit
	idem  *fakeIdem
	owner Caller
	mgr   Caller
}

const accountNo = "0123456789"

func newBankRig(t *testing.T) bankRig {
	t.Helper()
	ar := newAuthRig(t, allowAll{}, allowAll{})
	repo, audit, idem := newFakeBankRepo(), &fakeAudit{}, &fakeIdem{m: map[string]idemState{}}
	b := NewBank(ar.a.sessions.uow, repo, ar.a, fakeEnc{}, idem, audit, &seqIDs{}, ar.clock)
	return bankRig{b: b, repo: repo, audit: audit, idem: idem,
		owner: Caller{TenantID: "tn_a", UserID: "us_ann", Role: access.RoleOwner},
		mgr:   Caller{TenantID: "tn_a", UserID: "us_mgr", Role: access.RoleManager}}
}

func accountInput() BankAccountInput {
	return BankAccountInput{OwnerPin: ownerPin, BankBin: "970436", AccountNo: accountNo, AccountName: "CASA GUESTHOUSE", MakeDefaultWhenConnected: true}
}

func TestCreateBankAccount_PendingAndMasked_SG1001(t *testing.T) {
	r := newBankRig(t)
	ctx := context.Background()
	v, err := r.b.CreateAccount(ctx, r.owner, "k1", accountInput())
	if err != nil || v.SepayStatus != "PENDING" || v.IsDefault || v.BankName != "Vietcombank" || v.AccountNoMasked != "******6789" {
		t.Fatalf("%+v %v", v, err)
	}
	// The number is encrypted at rest and appears nowhere else: not in the audit row, not in the stored replay.
	if strings.Contains(string(r.repo.accounts[v.ID].AccountName), accountNo) {
		t.Fatal("number in view")
	}
	for _, e := range r.audit.entries {
		if strings.Contains(string(e.Before)+string(e.After)+e.EntityID, accountNo) {
			t.Fatalf("account number in audit: %+v", e)
		}
	}
	for _, st := range r.idem.snapshot() {
		if strings.Contains(string(st.body), accountNo) {
			t.Fatal("account number in the idempotency store")
		}
	}
	// A replay returns the same account; the same account under another key is a conflict.
	v2, err := r.b.CreateAccount(ctx, r.owner, "k1", accountInput())
	if err != nil || v2.ID != v.ID || len(r.repo.accounts) != 1 {
		t.Fatalf("replay: %+v %v", v2, err)
	}
	if _, err = r.b.CreateAccount(ctx, r.owner, "k2", accountInput()); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate account: %v", err)
	}
}

func TestBankAccount_OwnerPinRequired_SG1001(t *testing.T) {
	r := newBankRig(t)
	ctx := context.Background()
	in := accountInput()
	in.OwnerPin = pinWrong
	if _, err := r.b.CreateAccount(ctx, r.owner, "k1", in); !errors.Is(err, ErrOwnerPinInvalid) || len(r.repo.accounts) != 0 {
		t.Fatalf("create with wrong PIN: %v", err)
	}
	if got := r.b.auth.repo.(*fakeAuthRepo).users[authKey{"tn_a", "us_ann"}].Pin.FailedCount; got != 1 {
		t.Fatalf("a wrong owner PIN counts: %d", got)
	}
	v, _ := r.b.CreateAccount(ctx, r.owner, "k2", accountInput())
	if _, err := r.b.MakeDefault(ctx, r.owner, v.ID, pinWrong); !errors.Is(err, ErrOwnerPinInvalid) {
		t.Fatalf("make default: %v", err)
	}
	if err := r.b.RemoveAccount(ctx, r.owner, v.ID, pinWrong); !errors.Is(err, ErrOwnerPinInvalid) || len(r.repo.accounts) != 1 {
		t.Fatalf("remove: %v", err)
	}
	var ve *ValidationError
	if err := r.b.RemoveAccount(ctx, r.owner, v.ID, "12"); !errors.As(err, &ve) {
		t.Fatalf("bad PIN shape: %v", err)
	}
}

func TestBankAccount_DefaultRules_SG1001_Rule19(t *testing.T) {
	r := newBankRig(t)
	ctx := context.Background()
	a, _ := r.b.CreateAccount(ctx, r.owner, "k1", accountInput())
	in := accountInput()
	in.AccountNo = "9876543210"
	b, _ := r.b.CreateAccount(ctx, r.owner, "k2", in)
	// Only a connected account may become the default.
	if _, err := r.b.MakeDefault(ctx, r.owner, a.ID, ownerPin); !errors.Is(err, ErrConflict) {
		t.Fatalf("pending account as default: %v", err)
	}
	r.repo.accounts[a.ID].SepayStatus = statusConnected
	v, err := r.b.MakeDefault(ctx, r.owner, a.ID, ownerPin)
	if err != nil || !v.IsDefault {
		t.Fatalf("%+v %v", v, err)
	}
	// The default cannot be removed; a non-default can.
	if err = r.b.RemoveAccount(ctx, r.owner, a.ID, ownerPin); !errors.Is(err, ErrConflict) {
		t.Fatalf("remove default: %v", err)
	}
	if err = r.b.RemoveAccount(ctx, r.owner, b.ID, ownerPin); err != nil || len(r.repo.accounts) != 1 {
		t.Fatalf("remove other: %v", err)
	}
	if err = r.b.RemoveAccount(ctx, r.owner, "nope", ownerPin); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestBankAccount_InputAndRoles_SG1001(t *testing.T) {
	r := newBankRig(t)
	ctx := context.Background()
	for name, mut := range map[string]func(*BankAccountInput){
		"bin":     func(i *BankAccountInput) { i.BankBin = "97043" },
		"number":  func(i *BankAccountInput) { i.AccountNo = "12ab" },
		"name":    func(i *BankAccountInput) { i.AccountName = "" },
		"pin":     func(i *BankAccountInput) { i.OwnerPin = "1" },
		"toolong": func(i *BankAccountInput) { i.AccountName = strings.Repeat("A", 61) },
	} {
		in := accountInput()
		mut(&in)
		var ve *ValidationError
		if _, err := r.b.CreateAccount(ctx, r.owner, "k-"+name, in); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Owner only: the manager cannot touch bank accounts but may read the property and the SePay status.
	if _, err := r.b.CreateAccount(ctx, r.mgr, "k9", accountInput()); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("manager create: %v", err)
	}
	if _, err := r.b.ListAccounts(ctx, r.mgr); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("manager list: %v", err)
	}
	if _, err := r.b.GetProperty(ctx, r.mgr); err != nil {
		t.Errorf("manager property: %v", err)
	}
	if _, err := r.b.SepayStatus(ctx, r.mgr); err != nil {
		t.Errorf("manager status: %v", err)
	}
	if _, err := r.b.UpdateProperty(ctx, r.mgr, PropertyPatch{}); !errors.Is(err, access.ErrRoleForbidden) {
		t.Errorf("manager update property: %v", err)
	}
}

func TestUpdateProperty_Ranges_SG1001(t *testing.T) {
	r := newBankRig(t)
	ctx := context.Background()
	n := func(v int) *int { return &v }
	for name, p := range map[string]PropertyPatch{
		"qr low": {QRExpiryMinutes: n(4)}, "qr high": {QRExpiryMinutes: n(241)}, "retention": {IDRetentionDays: n(0)},
		"history": {FrontDeskHistoryDays: n(91)}, "name": {Name: new(string)},
	} {
		var ve *ValidationError
		if _, err := r.b.UpdateProperty(ctx, r.owner, p); !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
	v, err := r.b.UpdateProperty(ctx, r.owner, PropertyPatch{QRExpiryMinutes: n(45)})
	if err != nil || v.QRExpiryMinutes != 45 {
		t.Fatalf("%+v %v", v, err)
	}
}

// The installer's SetSecret stores the secret encrypted, connects the account and applies make-default-when-connected.
func TestInstallerSetSecret_SG703(t *testing.T) {
	ar := newAuthRig(t, allowAll{}, allowAll{})
	repo, audit, alerts := newFakeBankRepo(), &fakeAudit{}, &fakeAlerts{}
	inst := NewInstaller(ar.a.sessions.uow, fakeTenants{"casa": "tn_a"}, nil, nil, repo, nil, nil, fakeEnc{}, fakeHasher{}, fixedPin{issued}, fixedToken{"hook123"},
		audit, alerts, &seqIDs{}, ar.clock)
	b := NewBank(ar.a.sessions.uow, repo, ar.a, fakeEnc{}, &fakeIdem{m: map[string]idemState{}}, &fakeAudit{}, &seqIDs{}, ar.clock)
	owner := Caller{TenantID: "tn_a", UserID: "us_ann", Role: access.RoleOwner}
	v, err := b.CreateAccount(context.Background(), owner, "k1", accountInput())
	if err != nil {
		t.Fatal(err)
	}
	path, err := inst.WebhookPath(context.Background(), "casa")
	if err != nil || path != "/v1/webhooks/bank/hook123" {
		t.Fatalf("%q %v", path, err)
	}
	if again, _ := inst.WebhookPath(context.Background(), "casa"); again != path {
		t.Fatal("the hook id is stable")
	}
	if err = inst.SetSecret(context.Background(), "casa", "s3cret-value", ""); err != nil {
		t.Fatal(err)
	}
	a := repo.accounts[v.ID]
	if a.SepayStatus != "CONNECTED" || !a.IsDefault {
		t.Fatalf("connected and default: %+v", a)
	}
	if strings.Contains(string(repo.secret), "s3cret-value") {
		t.Fatal("the secret must be stored encrypted")
	}
	if len(alerts.raised) != 1 || alerts.raised[0].Kind != AlertSepayUpdated || len(audit.entries) != 1 || audit.entries[0].Action != "INSTALLER_SEPAY_UPDATED" {
		t.Fatalf("owner-visible traces: %+v %+v", alerts.raised, audit.entries)
	}
	if d := alerts.raised[0].Details; d["code"] != "SECRET_SET" || d["actor"] != "INSTALLER" || d["at"] == "" {
		t.Fatalf("the alert says what happened and who did it, not just a timestamp: %v", d)
	}
	for _, e := range audit.entries {
		if strings.Contains(string(e.After), "s3cret") {
			t.Fatal("secret in audit")
		}
	}
	// Two accounts need --account; an unknown guesthouse is refused.
	in := accountInput()
	in.AccountNo = "9876543210"
	if _, err = b.CreateAccount(context.Background(), owner, "k2", in); err != nil {
		t.Fatal(err)
	}
	if err = inst.SetSecret(context.Background(), "casa", "s3cret-value", ""); !errors.Is(err, ErrAccountChoice) {
		t.Fatalf("two accounts: %v", err)
	}
	if err = inst.SetSecret(context.Background(), "nowhere", "x", ""); !errors.Is(err, ErrTenantUnknown) {
		t.Fatalf("unknown tenant: %v", err)
	}
	if err = inst.SetSecret(context.Background(), "casa", "", ""); err == nil {
		t.Fatal("empty secret")
	}
}
