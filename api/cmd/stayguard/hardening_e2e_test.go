//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/pcaokhai/stayguard/api/internal/adapter/crypto"
)

// addStaff creates a receptionist as the owner and returns the id and the one-time PIN.
func (e *env) addStaff(owner, key, username string) (id, pin string) {
	e.t.Helper()
	body := staffBody()
	body["username"], body["name"] = username, strings.ToUpper(username[:1])+username[1:]
	st, raw := e.send("POST", "/v1/owner/staff", owner, key, body)
	if st != 201 {
		e.t.Fatalf("create staff %s: %d %s", username, st, raw)
	}
	m := parse(raw)
	return m["staff"].(map[string]any)["id"].(string), m["oneTimePin"].(map[string]any)["pin"].(string)
}

// settle replaces a one-time PIN by a chosen PIN and returns a normal session token.
func (e *env) settle(user, oneTime, pin string) {
	e.t.Helper()
	in := e.signIn(user, oneTime)
	if in.status != 200 {
		e.t.Fatalf("one-time sign-in: %d %v", in.status, in.body)
	}
	if r := e.call("PUT", "/v1/me/pin", in.str("accessToken"), map[string]any{"currentPin": oneTime, "newPin": pin}); r.status != 204 {
		e.t.Fatalf("set pin: %d %v", r.status, r.body)
	}
}

// Hardening 5: changing a PIN, locking and removing a user end that user's other sessions at once.
func TestSessionsRevoked_Hardening5(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	e.seedStayTenant(staffTenant, 1)
	owner := e.signIn("owner1", ownerPIN).str("accessToken")
	id, one := e.addStaff(owner, newKey(), "linh")
	e.settle("linh", one, "260814")
	alive := func(token string) bool { return e.call("GET", "/v1/me", token, nil).status == 200 }
	login := func() string { return e.signIn("linh", "260814").str("accessToken") }

	a, b := login(), login()
	if r := e.call("PUT", "/v1/me/pin", a, map[string]any{"currentPin": "260814", "newPin": "731902"}); r.status != 204 {
		t.Fatalf("change pin: %d %v", r.status, r.body)
	}
	if !alive(a) || alive(b) {
		t.Fatalf("changing the PIN keeps this session (%v) and ends the others (%v)", alive(a), alive(b))
	}
	a, b = e.signIn("linh", "731902").str("accessToken"), e.signIn("linh", "731902").str("accessToken")
	if r := e.call("POST", "/v1/owner/staff/"+id+"/lock", owner, nil); r.status != 200 {
		t.Fatalf("lock: %d", r.status)
	}
	if alive(a) || alive(b) {
		t.Fatal("locking ends every session at once")
	}
	e.call("POST", "/v1/owner/staff/"+id+"/unlock", owner, nil)
	a = e.signIn("linh", "731902").str("accessToken")
	if st, raw := e.send("POST", "/v1/owner/staff/"+id+"/pin-reset", owner, newKey(), nil); st != 200 || alive(a) {
		t.Fatalf("a PIN reset ends the sessions: %d %s", st, raw)
	}
	e.call("POST", "/v1/owner/staff/"+id+"/unlock", owner, nil)
	one2 := func() string {
		_, raw := e.send("POST", "/v1/owner/staff/"+id+"/pin-reset", owner, newKey(), nil)
		return parse(raw)["pin"].(string)
	}()
	e.settle("linh", one2, "846205")
	a = e.signIn("linh", "846205").str("accessToken")
	if r := e.call("POST", "/v1/owner/staff/"+id+"/remove", owner, map[string]any{"ownerPin": ownerPIN}); r.status != 204 || alive(a) {
		t.Fatalf("removing ends the sessions: %d", r.status)
	}
}

// Hardening 6: nothing logged across sign-in, PIN change, PIN reset and staff creation holds a PIN, the key or the pepper.
func TestLogsHoldNoPinOrPepper_Hardening6(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	e.seedStayTenant(staffTenant, 1)
	owner := e.signIn("owner1", ownerPIN).str("accessToken")
	_ = e.signIn("owner1", "159357") // a wrong PIN
	id, one := e.addStaff(owner, newKey(), "linh")
	e.settle("linh", one, "260814")
	staff := e.signIn("linh", "260814").str("accessToken")
	_ = e.call("PUT", "/v1/me/pin", staff, map[string]any{"currentPin": "260814", "newPin": "731902"})
	_ = e.call("PUT", "/v1/me/pin", staff, map[string]any{"currentPin": "000000", "newPin": "846205"}) // wrong current
	_, raw := e.send("POST", "/v1/owner/staff/"+id+"/pin-reset", owner, newKey(), nil)
	reset, _ := parse(raw)["pin"].(string)
	_ = e.call("POST", "/v1/owner/staff/"+id+"/remove", owner, map[string]any{"ownerPin": "159357"}) // wrong owner PIN

	pepper := crypto.DerivePinPepper(testDataKey)
	secrets := map[string]string{
		"owner PIN": ownerPIN, "wrong PIN": "159357", "one-time PIN": one, "chosen PIN": "260814", "changed PIN": "731902",
		"reset PIN": reset, "mistyped PIN": "000000", "key (base64)": base64.StdEncoding.EncodeToString(testDataKey),
		"key (hex)": hex.EncodeToString(testDataKey), "pepper (hex)": hex.EncodeToString(pepper), "pepper (base64)": base64.StdEncoding.EncodeToString(pepper),
		"tokens": owner, "staff token": staff,
	}
	logs := e.logs.String()
	if logs == "" {
		t.Fatal("expected request logs")
	}
	for name, s := range secrets {
		if s != "" && strings.Contains(logs, s) {
			t.Errorf("%s found in the server logs", name)
		}
	}
	if bytes.Contains([]byte(logs), []byte("$2a$")) || strings.Contains(logs, "p1$") {
		t.Error("a PIN hash reached the logs")
	}
}

// Hardening 10: a wrong guesthouse code, a wrong user and a wrong PIN get the same answer, byte for byte.
func TestSignInAnswersAreIdentical_Hardening10(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	e.seedStayTenant(staffTenant, 1)
	cases := map[string]map[string]any{
		"wrong code": {"guesthouseCode": "nowhere", "username": "owner1", "pin": ownerPIN},
		"wrong user": {"guesthouseCode": staffCode, "username": "ghost", "pin": ownerPIN},
		"wrong pin":  {"guesthouseCode": staffCode, "username": "owner1", "pin": "159357"},
	}
	var first []byte
	for name, body := range cases {
		st, raw := e.send("POST", "/v1/auth/sign-in", "", "", body)
		if st != http.StatusUnauthorized || parse(raw)["code"] != "PIN_INVALID" {
			t.Fatalf("%s: %d %s", name, st, raw)
		}
		if first == nil {
			first = raw
			continue
		}
		if !bytes.Equal(first, raw) {
			t.Errorf("%s answers differently:\n%s\n%s", name, first, raw)
		}
	}
}

// Hardening 1 end to end: a hash from before the pepper still signs in and is replaced by a peppered one.
func TestLegacyPinHashUpgradedOnSignIn_Hardening1(t *testing.T) {
	e := newEnv(t)
	e.seedOwner()
	legacy, err := bcrypt.GenerateFromPassword([]byte(ownerPIN), 4)
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE app.pin_credentials SET pin_hash = $1 WHERE user_id = 'us_owner'`, string(legacy))
	if r := e.signIn("owner1", "159357"); r.status != 401 {
		t.Fatalf("wrong PIN against a legacy hash: %d", r.status)
	}
	if r := e.signIn("owner1", ownerPIN); r.status != 200 {
		t.Fatalf("legacy hash must still verify: %d %v", r.status, r.body)
	}
	var h string
	if err = e.owner.QueryRow(context.Background(), `SELECT pin_hash FROM app.pin_credentials WHERE user_id = 'us_owner'`).Scan(&h); err != nil || !strings.HasPrefix(h, "p1$") {
		t.Fatalf("hash not upgraded: %q %v", h, err)
	}
	if r := e.signIn("owner1", ownerPIN); r.status != 200 {
		t.Fatalf("sign in with the upgraded hash: %d", r.status)
	}
}
