package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	// webhookProvider names SePay events in payment_events; the dedupe key is (provider, SePay transaction id).
	webhookProvider = "sepay"
	// webhookTolerance is the replay window SePay documents for X-SePay-Timestamp.
	webhookTolerance = 300 * time.Second
	signaturePrefix  = "sha256="
	// maxWebhookContent bounds the transfer note kept for matching and alerts.
	maxWebhookContent = 500
)

var (
	// ErrWebhookUnknown: no tenant has that hook id (HTTP 404).
	ErrWebhookUnknown = errors.New("unknown webhook")
	// ErrWebhookRejected: the signature, timestamp or secret is not right (HTTP 401, no detail).
	ErrWebhookRejected = errors.New("webhook rejected")
)

// sepayPayload is the SePay webhook body (https://developer.sepay.vn/vi/sepay-webhooks/tich-hop-webhook). Unknown
// fields are ignored. The provider's own `code` field is not read: the bill code is found inside the content.
type sepayPayload struct {
	ID              int64  `json:"id"`
	TransactionDate string `json:"transactionDate"`
	AccountNumber   string `json:"accountNumber"`
	Content         string `json:"content"`
	TransferType    string `json:"transferType"`
	TransferAmount  int64  `json:"transferAmount"`
}

// Webhook receives SePay events: tenant by hook id, signature over the raw body, then the one settlement handler.
type Webhook struct {
	tenants  TenantByHook
	uow      UnitOfWork
	bank     BankRepo
	payments *Payments
	enc      Encryptor
	clock    Clock
}

func NewWebhook(tenants TenantByHook, uow UnitOfWork, bank BankRepo, payments *Payments, enc Encryptor, clock Clock) *Webhook {
	return &Webhook{tenants, uow, bank, payments, enc, clock}
}

// SepaySignature is what SePay sends in X-SePay-Signature: sha256= and the hex HMAC-SHA256 of "{timestamp}.{raw body}".
func SepaySignature(secret []byte, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(timestamp + "."))
	m.Write(body)
	return signaturePrefix + hex.EncodeToString(m.Sum(nil))
}

// Receive checks and processes one webhook. A nil error means answer 200 {"success": true}, duplicates included.
func (w *Webhook) Receive(ctx context.Context, hookID, signature, timestamp string, body []byte) error {
	tenantID, found, err := w.tenants.TenantByHook(ctx, hookID)
	if err != nil {
		return err
	}
	if !found {
		return ErrWebhookUnknown
	}
	ctx = context.WithoutCancel(ctx) // SePay retrying a half-processed event is safe, but never drop a state change
	var secret []byte
	var accounts []AccountBlob
	err = w.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error {
		enc, err := w.bank.SecretEnc(ctx, tx)
		if err != nil || len(enc) == 0 {
			return errOr(err, ErrWebhookRejected)
		}
		if secret, err = w.enc.Decrypt(tenantID, sepaySecretField, enc); err != nil {
			return err
		}
		accounts, err = w.bank.AccountBlobs(ctx, tx)
		return err
	})
	if err != nil {
		return err
	}
	if !w.signatureValid(secret, signature, timestamp, body) {
		_ = w.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error { return w.bank.SetSignatureOK(ctx, tx, false) })
		return ErrWebhookRejected
	}
	var p sepayPayload
	if err = json.Unmarshal(body, &p); err != nil || p.ID <= 0 || p.TransferAmount <= 0 || (p.TransferType != "in" && p.TransferType != "out") {
		return &ValidationError{Field: "body", Reason: "not a SePay transaction"}
	}
	if err = w.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error { return w.bank.SetSignatureOK(ctx, tx, true) }); err != nil {
		return err
	}
	ev := PaymentEvent{TenantID: tenantID, Provider: webhookProvider, ExternalID: strconv.FormatInt(p.ID, 10), Amount: p.TransferAmount,
		Content: clipTo(p.Content, maxWebhookContent), ReceivedAt: w.receivedAt(p.TransactionDate)}
	accountID, ours := w.ourAccount(tenantID, accounts, p.AccountNumber)
	if ours {
		err = w.uow.Do(ctx, tenantID, func(ctx context.Context, tx Tx) error { return w.bank.TouchWebhook(ctx, tx, accountID, ev.ReceivedAt) })
		if err != nil {
			return err
		}
	}
	if p.TransferType != "in" || !ours { // outgoing money and other accounts' money settle nothing
		return w.payments.RecordIgnored(ctx, ev)
	}
	_, err = w.payments.Settle(ctx, ev)
	return err
}

// signatureValid checks the timestamp window and the HMAC with a constant-time compare.
func (w *Webhook) signatureValid(secret []byte, signature, timestamp string, body []byte) bool {
	ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return false
	}
	delta := w.clock.Now().Sub(time.Unix(ts, 0))
	if delta > webhookTolerance || delta < -webhookTolerance {
		return false
	}
	want := SepaySignature(secret, strings.TrimSpace(timestamp), body)
	return hmac.Equal([]byte(want), []byte(signature))
}

// ourAccount says whether accountNumber is one of the tenant's receiving accounts.
func (w *Webhook) ourAccount(tenantID string, accounts []AccountBlob, number string) (string, bool) {
	if number == "" {
		return "", false
	}
	for _, a := range accounts {
		plain, err := w.enc.Decrypt(tenantID, bankAccountFieldOf(a.ID), a.Enc)
		if err != nil {
			continue
		}
		var acc struct{ AccountNo string }
		if json.Unmarshal(plain, &acc) == nil && acc.AccountNo == number {
			return a.ID, true
		}
	}
	return "", false
}

// receivedAt reads SePay's Vietnam-time date; a value that does not parse falls back to the server clock.
func (w *Webhook) receivedAt(s string) time.Time {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err == nil {
		if t, perr := time.ParseInLocation("2006-01-02 15:04:05", s, loc); perr == nil {
			return storedTime(t)
		}
	}
	return storedTime(w.clock.Now())
}

func clipTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
