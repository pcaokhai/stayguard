//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	hookSecret = "test-hmac-secret-not-real"
	hookID     = "hook_test_0123456789abcdef"
)

// sepaySample is the payload printed in SePay's webhook documentation, unchanged.
func sepaySample(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/sepay_webhook_in.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// connectHook gives the rig's tenant a webhook id and an encrypted secret, as the installer CLI would.
func (r payRig) connectHook(secret string) {
	r.e.t.Helper()
	enc, err := crypto.NewAESGCM(testDataKey)
	if err != nil {
		r.e.t.Fatal(err)
	}
	blob, err := enc.Encrypt(r.tenant, "sepay_secret", []byte(secret))
	if err != nil {
		r.e.t.Fatal(err)
	}
	r.e.exec(`UPDATE app.tenants SET hook_id = $2, sepay_secret_enc = $3 WHERE id = $1`, r.tenant, hookID, blob)
}

// webhook posts a body signed the way SePay does: sha256= and the hex HMAC of "{timestamp}.{raw body}".
func (r payRig) webhook(hook, secret string, ts time.Time, body []byte, signature string) (int, string) {
	r.e.t.Helper()
	stamp := strconv.FormatInt(ts.Unix(), 10)
	if signature == "" {
		signature = app.SepaySignature([]byte(secret), stamp, body)
	}
	req, _ := http.NewRequest("POST", r.e.srv.URL+"/v1/webhooks/bank/"+hook, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SePay-Signature", signature)
	req.Header.Set("X-SePay-Timestamp", stamp)
	res, err := http.DefaultClient.Do(req) // no Authorization header: the signature is the credential
	if err != nil {
		r.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out)
}

func (r payRig) sepayBody(t *testing.T, id int, kind string, amount int64, content string) []byte {
	m := sepaySample(t)
	m["id"], m["transferType"], m["transferAmount"], m["content"] = id, kind, amount, content
	m["accountNumber"] = "0000000000" // the tenant's own account (the demo seed)
	m["code"] = "SEVN-NOT-THE-BILL-CODE"
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (r payRig) events(result string) int {
	return r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND provider = 'sepay' AND result = $2`, r.tenant, result)
}

// SG-703 AC3 and the P3 list: signature, dedupe on SePay's id, incoming only, bill code found in the content.
func TestSepayWebhook_SG703(t *testing.T) {
	r := newPayRig(t, "A102")
	r.connectHook(hookSecret)
	if st, p := r.pay("TRANSFER"); st != 201 {
		t.Fatalf("create transfer: %d %v", st, p)
	}
	now := r.e.clock.Now()

	// The documentation sample, signed with the test secret, verifies (no code of ours in it: unmatched).
	sample, _ := json.Marshal(sepaySample(t))
	if st, body := r.webhook(hookID, hookSecret, now, sample, ""); st != 200 || body != `{"success": true}` {
		t.Fatalf("documented sample: %d %s", st, body)
	}
	if r.events("UNMATCHED") != 1 || r.status("invoices", r.invoice) != "OPEN" {
		t.Fatal("a transfer for another account stays unmatched and settles nothing")
	}

	// Bad signature, tampered body, wrong secret, stale timestamp, unknown hook: nothing is stored.
	good := r.sepayBody(t, 1001, "in", r.balance, "CK "+r.code+" thanh toan")
	tampered := bytes.Replace(good, []byte(strconv.FormatInt(r.balance, 10)), []byte("1"), 1)
	signed := app.SepaySignature([]byte(hookSecret), strconv.FormatInt(now.Unix(), 10), good)
	for name, c := range map[string]struct {
		hook, secret string
		ts           time.Time
		body         []byte
		sig          string
	}{
		"garbage signature": {hookID, hookSecret, now, good, "sha256=00"},
		"no prefix":         {hookID, hookSecret, now, good, strings.TrimPrefix(signed, "sha256=")},
		"tampered body":     {hookID, hookSecret, now, tampered, signed},
		"wrong secret":      {hookID, "other-secret", now, good, ""},
		"stale timestamp":   {hookID, hookSecret, now.Add(-6 * time.Minute), good, ""},
		"future timestamp":  {hookID, hookSecret, now.Add(6 * time.Minute), good, ""},
	} {
		if st, body := r.webhook(c.hook, c.secret, c.ts, c.body, c.sig); st != 401 || strings.Contains(body, "success") || strings.Contains(strings.ToLower(body), "signature") {
			t.Errorf("%s: %d %s", name, st, body)
		}
	}
	if st, _ := r.webhook("nope", hookSecret, now, good, ""); st != 404 {
		t.Errorf("unknown hook: %d", st)
	}
	if r.events("SETTLED")+r.events("UNMATCHED")+r.events("MISMATCH") != 1 || r.status("invoices", r.invoice) != "OPEN" {
		t.Fatal("a rejected webhook must store and settle nothing")
	}
	if n := r.e.count(`SELECT count(*) FROM app.tenants WHERE id = $1 AND sepay_signature_ok = false`, r.tenant); n != 1 {
		t.Error("a failed check is visible in the status")
	}

	// Outgoing money with the right code and amount is stored as UNMATCHED and ignored, and raises no alert.
	alerts := r.e.count(`SELECT count(*) FROM app.alerts WHERE tenant_id = $1`, r.tenant)
	out := r.sepayBody(t, 1002, "out", r.balance, "CK "+r.code)
	if st, body := r.webhook(hookID, hookSecret, now, out, ""); st != 200 || body != `{"success": true}` {
		t.Fatalf("outgoing: %d %s", st, body)
	}
	if r.status("invoices", r.invoice) != "OPEN" || r.events("UNMATCHED") != 2 || r.e.count(`SELECT count(*) FROM app.alerts WHERE tenant_id = $1`, r.tenant) != alerts {
		t.Fatal("outgoing transfers settle nothing and raise nothing")
	}
	// Money to an account that is not the tenant's settles nothing either.
	other := bytes.Replace(good, []byte(`"0000000000"`), []byte(`"9999999999"`), 1)
	if st, _ := r.webhook(hookID, hookSecret, now, other, ""); st != 200 || r.status("invoices", r.invoice) != "OPEN" {
		t.Fatalf("other account: %d", st)
	}

	// The valid incoming transfer settles; the bill code is found inside the content, not taken from `code`.
	good2 := r.sepayBody(t, 1003, "in", r.balance, "chuyen khoan "+strings.ToLower(r.code)+" tien phong")
	if st, body := r.webhook(hookID, hookSecret, now, good2, ""); st != 200 || body != `{"success": true}` {
		t.Fatalf("valid: %d %s", st, body)
	}
	if r.status("invoices", r.invoice) != "PAID" || r.roomStatus() != "TO_CLEAN" || r.events("SETTLED") != 1 {
		t.Fatalf("invoice %s room %s settled %d", r.status("invoices", r.invoice), r.roomStatus(), r.events("SETTLED"))
	}
	if n := r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND provider = 'sepay' AND external_id = '1003'`, r.tenant); n != 1 {
		t.Fatal("the event is keyed by SePay's transaction id")
	}
	// A retry of the same transaction (same id) gets the same answer and changes nothing.
	for i := 0; i < 2; i++ {
		if st, body := r.webhook(hookID, hookSecret, now, good2, ""); st != 200 || body != `{"success": true}` {
			t.Fatalf("retry %d: %d %s", i, st, body)
		}
	}
	if r.events("SETTLED") != 1 || r.e.count(`SELECT count(*) FROM app.payment_events WHERE tenant_id = $1 AND external_id = '1003'`, r.tenant) != 1 {
		t.Fatal("a retry must not settle or store twice")
	}
	if n := r.e.count(`SELECT count(*) FROM app.tenants WHERE id = $1 AND sepay_signature_ok = true`, r.tenant); n != 1 {
		t.Error("a valid check is visible in the status")
	}
	if n := r.e.count(`SELECT count(*) FROM app.bank_accounts WHERE tenant_id = $1 AND last_webhook_at IS NOT NULL`, r.tenant); n != 1 {
		t.Error("the receiving account shows its last webhook")
	}
	if strings.Contains(r.e.logs.String(), hookSecret) || strings.Contains(r.e.logs.String(), "0000000000") || strings.Contains(r.e.logs.String(), "NGUYEN VAN A") {
		t.Error("secret, account number or payer name in the logs")
	}
	if strings.Contains(r.e.dumpTable("tenants"), hookSecret) {
		t.Error("the secret is stored in clear")
	}
}

// Until the installer has set a secret nothing is accepted.
func TestSepayWebhook_NoSecretRejects_SG703(t *testing.T) {
	r := newPayRig(t, "A102")
	r.e.exec(`UPDATE app.tenants SET hook_id = $2 WHERE id = $1`, r.tenant, hookID)
	b, _ := json.Marshal(sepaySample(t))
	if st, _ := r.webhook(hookID, hookSecret, r.e.clock.Now(), b, ""); st != 401 {
		t.Fatalf("no secret: %d", st)
	}
	if st, _ := r.webhook(hookID, hookSecret, r.e.clock.Now(), []byte("not json"), ""); st != 401 {
		t.Fatalf("junk without a secret: %d", st)
	}
}
