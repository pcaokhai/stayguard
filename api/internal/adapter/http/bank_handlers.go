package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// BankService is the property, receiving account and SePay status surface the handlers need.
type BankService interface {
	GetProperty(ctx context.Context, c app.Caller) (app.PropertyView, error)
	UpdateProperty(ctx context.Context, c app.Caller, p app.PropertyPatch) (app.PropertyView, error)
	ListAccounts(ctx context.Context, c app.Caller) ([]app.BankAccountView, error)
	CreateAccount(ctx context.Context, c app.Caller, key string, in app.BankAccountInput) (app.BankAccountView, error)
	MakeDefault(ctx context.Context, c app.Caller, id, ownerPin string) (app.BankAccountView, error)
	RemoveAccount(ctx context.Context, c app.Caller, id, ownerPin string) error
	SepayStatus(ctx context.Context, c app.Caller) (app.SepayState, error)
}

// WithBank adds the property and bank use cases; the router always sets them.
func (s Server) WithBank(b BankService) Server { s.bank = b; return s }

func toProperty(v app.PropertyView) gen.Property {
	return gen.Property{GuesthouseCode: v.GuesthouseCode, Name: v.Name, Address: v.Address, Phone: v.Phone, QrExpiryMinutes: v.QRExpiryMinutes,
		IdRetentionDays: &v.IDRetentionDays, FrontDeskHistoryDays: &v.FrontDeskHistoryDays}
}

func toBankAccount(v app.BankAccountView) gen.BankAccount {
	return gen.BankAccount{Id: v.ID, BankBin: v.BankBin, BankName: v.BankName, AccountNoMasked: v.AccountNoMasked, AccountName: v.AccountName,
		IsDefault: v.IsDefault, SepayStatus: gen.BankAccountSepayStatus(v.SepayStatus), LastWebhookAt: v.LastWebhookAt}
}

func (s Server) GetProperty(ctx context.Context, _ gen.GetPropertyRequestObject) (gen.GetPropertyResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.bank.GetProperty(ctx, c)
	if err != nil {
		return nil, err
	}
	return gen.GetProperty200JSONResponse(toProperty(v)), nil
}

func (s Server) UpdateProperty(ctx context.Context, req gen.UpdatePropertyRequestObject) (gen.UpdatePropertyResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "required"}
	}
	b := req.Body
	v, err := s.bank.UpdateProperty(ctx, c, app.PropertyPatch{Name: b.Name, Address: b.Address, Phone: b.Phone,
		QRExpiryMinutes: b.QrExpiryMinutes, IDRetentionDays: b.IdRetentionDays, FrontDeskHistoryDays: b.FrontDeskHistoryDays})
	if err != nil {
		return nil, err
	}
	return gen.UpdateProperty200JSONResponse(toProperty(v)), nil
}

func (s Server) ListBankAccounts(ctx context.Context, _ gen.ListBankAccountsRequestObject) (gen.ListBankAccountsResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	vs, err := s.bank.ListAccounts(ctx, c)
	if err != nil {
		return nil, err
	}
	items := make([]gen.BankAccount, len(vs))
	for i, v := range vs {
		items[i] = toBankAccount(v)
	}
	return gen.ListBankAccounts200JSONResponse{Items: items}, nil
}

func (s Server) CreateBankAccount(ctx context.Context, req gen.CreateBankAccountRequestObject) (gen.CreateBankAccountResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || b.OwnerPin == nil || b.AccountNo == nil {
		return nil, &app.ValidationError{Field: "body", Reason: "ownerPin and accountNo required"}
	}
	in := app.BankAccountInput{OwnerPin: *b.OwnerPin, BankBin: b.BankBin, AccountNo: *b.AccountNo, AccountName: b.AccountName}
	if b.MakeDefaultWhenConnected != nil {
		in.MakeDefaultWhenConnected = *b.MakeDefaultWhenConnected
	}
	v, err := s.bank.CreateAccount(ctx, c, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.CreateBankAccount201JSONResponse(toBankAccount(v)), nil
}

func (s Server) MakeDefaultBankAccount(ctx context.Context, req gen.MakeDefaultBankAccountRequestObject) (gen.MakeDefaultBankAccountResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.OwnerPin == nil {
		return nil, &app.ValidationError{Field: "ownerPin", Reason: "required"}
	}
	v, err := s.bank.MakeDefault(ctx, c, req.AccountId, *req.Body.OwnerPin)
	if err != nil {
		return nil, err
	}
	return gen.MakeDefaultBankAccount200JSONResponse(toBankAccount(v)), nil
}

func (s Server) RemoveBankAccount(ctx context.Context, req gen.RemoveBankAccountRequestObject) (gen.RemoveBankAccountResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.OwnerPin == nil {
		return nil, &app.ValidationError{Field: "ownerPin", Reason: "required"}
	}
	if err = s.bank.RemoveAccount(ctx, c, req.AccountId, *req.Body.OwnerPin); err != nil {
		return nil, err
	}
	return gen.RemoveBankAccount204Response{}, nil
}

func (s Server) GetSepayStatus(ctx context.Context, _ gen.GetSepayStatusRequestObject) (gen.GetSepayStatusResponseObject, error) {
	c, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.bank.SepayStatus(ctx, c)
	if err != nil {
		return nil, err
	}
	out := gen.SepayStatus{Status: gen.SepayStatusStatusNOTCONNECTED, LastWebhookAt: st.LastWebhookAt, SignatureValid: st.SignatureOK}
	if st.DefaultConnected {
		out.Status = gen.SepayStatusStatusCONNECTED
	}
	return gen.GetSepayStatus200JSONResponse(out), nil
}

// ReceiveBankWebhookLegacy keeps stale provider settings failing loudly: the single-path webhook is gone.
func (Server) ReceiveBankWebhookLegacy(context.Context, gen.ReceiveBankWebhookLegacyRequestObject) (gen.ReceiveBankWebhookLegacyResponseObject, error) {
	return gen.ReceiveBankWebhookLegacy410ApplicationProblemPlusJSONResponse{Type: "about:blank", Title: "Gone", Status: 410, Code: "GONE"}, nil
}
