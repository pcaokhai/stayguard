//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const (
	gidNumber = "079203001234"
	gidMarker = "GPS-MARKER-10.7769N-106.7009E"
)

// photoJPEG is a small JPEG with an EXIF segment that carries a location marker.
func photoJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	seg := append([]byte("Exif\x00\x00MM\x00\x2a\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00"), []byte(gidMarker)...)
	app1 := append([]byte{0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}, seg...)
	raw := b.Bytes()
	return append(append(append([]byte{}, raw[:2]...), app1...), raw[2:]...)
}

func (e *env) upload(token, stayID, side string, file []byte, contentType string, consent *bool) (int, http.Header, []byte) {
	e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="card"`)
	h.Set("Content-Type", contentType)
	part, _ := mw.CreatePart(h)
	_, _ = part.Write(file)
	if consent != nil {
		_ = mw.WriteField("consent", map[bool]string{true: "true", false: "false"}[*consent])
	}
	_ = mw.Close()
	req, _ := http.NewRequest("PUT", e.srv.URL+"/v1/stays/"+stayID+"/guest-id/photos/"+side, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", newKey())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, raw
}

func (e *env) raw(method, path, token string) (int, http.Header, []byte) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, raw
}

// roleToken adds a staff member with the given app access and level on the stay building and returns a session token.
func (e *env) roleToken(owner, username, access, position, level string) string {
	e.t.Helper()
	body := staffBody()
	body["username"], body["name"], body["appAccess"], body["position"] = username, username, access, position
	body["buildingAccess"] = []map[string]any{{"buildingId": stayBuilding, "level": level}}
	st, raw := e.send("POST", "/v1/owner/staff", owner, newKey(), body)
	if st != 201 {
		e.t.Fatalf("create %s: %d %s", username, st, raw)
	}
	e.settle(username, parse(raw)["oneTimePin"].(map[string]any)["pin"].(string), "260814")
	return e.signIn(username, "260814").str("accessToken")
}

// SG-805 AC1-AC4 on the real stack.
func TestGuestID_Lifecycle_SG805(t *testing.T) {
	e := newEnv(t)
	owner := e.ownerSetup()
	desk := e.roleToken(owner, "linh", "RECEPTIONIST", "FRONT_DESK", "EDIT")
	viewer := e.roleToken(owner, "viewer", "RECEPTIONIST", "FRONT_DESK", "VIEW")
	clean := e.roleToken(owner, "hana", "HOUSEKEEPING", "HOUSEKEEPING", "EDIT")
	mgr := e.roleToken(owner, "mai2", "MANAGER", "MANAGER", "VIEW")
	yes := true

	// AC1: the number needs consent, at check-in and later.
	st, raw := e.checkIn(desk, 1, newKey(), stayBody(map[string]any{"idNumber": gidNumber}))
	if st != 422 || !strings.Contains(string(raw), "ID_CONSENT_REQUIRED") {
		t.Fatalf("check-in without consent: %d %s", st, raw)
	}
	st, raw = e.checkIn(desk, 1, newKey(), stayBody(nil))
	stayID := parse(raw)["id"].(string)
	if st != 201 || parse(raw)["guestId"].(map[string]any)["hasIdNumber"] != false {
		t.Fatalf("check-in without a number: %d %s", st, raw)
	}
	if st, raw = e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", desk, newKey(), map[string]any{"idNumber": gidNumber, "consent": false}); st != 422 {
		t.Fatalf("set without consent: %d %s", st, raw)
	}
	if st, raw = e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", desk, newKey(), map[string]any{"idNumber": "12ab", "consent": true}); st != 422 {
		t.Fatalf("bad number: %d %s", st, raw)
	}
	st, raw = e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", desk, newKey(), map[string]any{"idNumber": gidNumber, "consent": true})
	if st != 200 || parse(raw)["hasIdNumber"] != true || strings.Contains(string(raw), gidNumber) {
		t.Fatalf("set number: %d %s", st, raw)
	}
	var blob []byte
	if err := e.owner.QueryRow(context.Background(), `SELECT number_enc FROM app.guest_ids WHERE stay_id = $1`, stayID).Scan(&blob); err != nil || len(blob) == 0 || bytes.Contains(blob, []byte(gidNumber)) {
		t.Fatalf("the number must be stored encrypted: %v", err)
	}

	// Photos: consent is on file; EXIF is stripped, the image is re-encoded and stored encrypted.
	src := photoJPEG(t, 64, 40)
	st, _, raw = e.upload(desk, stayID, "FRONT", src, "image/jpeg", nil)
	if st != 200 || parse(raw)["hasFrontPhoto"] != true || parse(raw)["hasBackPhoto"] != false || parse(raw)["hasIdNumber"] != true {
		t.Fatalf("upload: %d %s", st, raw)
	}
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	if st, _, raw = e.upload(desk, stayID, "BACK", pngBuf.Bytes(), "image/png", nil); st != 200 || parse(raw)["hasBackPhoto"] != true {
		t.Fatalf("png upload: %d %s", st, raw)
	}
	var enc []byte
	_ = e.owner.QueryRow(context.Background(), `SELECT image_enc FROM app.guest_id_photos WHERE stay_id = $1 AND side = 'FRONT'`, stayID).Scan(&enc)
	if len(enc) == 0 || bytes.Contains(enc, []byte(gidMarker)) || bytes.HasPrefix(enc, []byte{0xFF, 0xD8}) {
		t.Fatal("the photo must be stored encrypted")
	}
	if st, _, _ = e.upload(desk, stayID, "FRONT", []byte("GIF89a not an image"), "image/gif", nil); st != 415 {
		t.Fatalf("unsupported media: %d", st)
	}
	if st, _, raw = e.upload(desk, stayID, "FRONT", make([]byte, 5<<20+10), "image/jpeg", nil); st != 413 || !strings.Contains(string(raw), "PHOTO_TOO_LARGE") {
		t.Fatalf("too large: %d %s", st, raw)
	}
	if st, _, _ = e.upload(desk, stayID, "FRONT", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 30)...), "image/jpeg", nil); st != 422 {
		t.Fatalf("broken image: %d", st)
	}
	// A photo for a stay with no consent on file needs the consent flag.
	st, raw = e.checkIn(desk, 2, newKey(), stayBody(nil))
	other := parse(raw)["id"].(string)
	if st, _, _ = e.upload(desk, other, "FRONT", src, "image/jpeg", nil); st != 422 {
		t.Fatalf("photo without consent: %d", st)
	}
	if st, _, _ = e.upload(desk, other, "FRONT", src, "image/jpeg", &yes); st != 200 {
		t.Fatalf("photo with consent: %d", st)
	}

	// AC2: front desk and housekeeping see indicators only, everywhere a stay appears.
	for name, token := range map[string]string{"receptionist": desk, "housekeeping": clean, "owner": owner} {
		st, _, body := e.raw("GET", "/v1/stays/"+stayID, token)
		if name == "housekeeping" {
			if st != 403 { // housekeeping has no stay access at all
				t.Errorf("housekeeping getStay: %d", st)
			}
			continue
		}
		g := parse(body)["guestId"].(map[string]any)
		if st != 200 || g["hasIdNumber"] != true || g["hasFrontPhoto"] != true || strings.Contains(string(body), gidNumber) || strings.Contains(string(body), "idNumberMasked") {
			t.Errorf("%s getStay: %d %s", name, st, body)
		}
	}

	// AC3: the record is for owner and manager only; a manager needs to see the building.
	path := "/v1/owner/stays/" + stayID + "/guest-id"
	for name, token := range map[string]string{"receptionist": desk, "viewer": viewer, "housekeeping": clean} {
		for _, p := range []struct{ method, path string }{{"GET", path}, {"POST", path + "/reveal"}, {"GET", path + "/photos/FRONT"}, {"DELETE", path + "/photos/FRONT"}, {"DELETE", path + "/number"}} {
			if st, _, body := e.raw(p.method, p.path, token); st != 403 || !strings.Contains(string(body), "ROLE_FORBIDDEN") {
				t.Errorf("%s %s %s: %d %s", name, p.method, p.path, st, body)
			}
		}
	}
	for _, tc := range []struct{ name, token string }{{"owner", owner}, {"manager", mgr}} {
		st, _, body := e.raw("GET", path, tc.token)
		rec := parse(body)
		if st != 200 || rec["idNumberMasked"] != "079******234" || rec["front"] == nil || rec["back"] == nil || rec["consentAt"] == nil || rec["deleteAfter"] != nil || strings.Contains(string(body), gidNumber) {
			t.Errorf("%s record: %d %s", tc.name, st, body)
		}
	}
	e.exec(`DELETE FROM app.building_permissions WHERE user_id = (SELECT id FROM app.users WHERE username = 'mai2')`)
	if st, _, body := e.raw("GET", path, mgr); st != 403 || !strings.Contains(string(body), "BUILDING_FORBIDDEN") {
		t.Errorf("a manager without the building: %d %s", st, body)
	}
	e.exec(`INSERT INTO app.building_permissions (tenant_id, user_id, building_id, level) SELECT $1, id, $2, 'VIEW' FROM app.users WHERE username = 'mai2'`, staffTenant, stayBuilding)
	if st, _, _ := e.raw("GET", "/v1/owner/stays/nope/guest-id", owner); st != 404 {
		t.Errorf("unknown stay: %d", st)
	}
	// A receptionist who may only view the building cannot add ID data.
	if st, _ = e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", viewer, newKey(), map[string]any{"idNumber": gidNumber, "consent": true}); st != 403 {
		t.Errorf("viewer writes: %d", st)
	}
	if st, _ = e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", clean, newKey(), map[string]any{"idNumber": gidNumber, "consent": true}); st != 403 {
		t.Errorf("housekeeping writes: %d", st)
	}

	// AC4: reveal, view, download and both deletes are audited; every response is no-store; the image is clean.
	st, hdr, body := e.raw("POST", path+"/reveal", mgr)
	if st != 200 || parse(body)["idNumber"] != gidNumber || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("reveal: %d %s %v", st, body, hdr)
	}
	st, hdr, img := e.raw("GET", path+"/photos/FRONT", owner)
	if st != 200 || hdr.Get("Content-Type") != "image/jpeg" || hdr.Get("Cache-Control") != "no-store" || hdr.Get("Content-Disposition") != "" {
		t.Fatalf("view: %d %v", st, hdr)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(img))
	if err != nil || decoded.Bounds().Dx() != 64 || bytes.Contains(img, []byte("Exif")) || bytes.Contains(img, []byte(gidMarker)) {
		t.Fatalf("the served photo must be a clean JPEG: %v", err)
	}
	if st, hdr, _ = e.raw("GET", path+"/photos/BACK?download=true", owner); st != 200 || !strings.HasPrefix(hdr.Get("Content-Disposition"), "attachment") || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("download: %d %v", st, hdr)
	}
	if st, hdr, _ = e.raw("GET", path+"/photos/SIDEWAYS", owner); st == 200 {
		t.Fatal("unknown side")
	}
	if st, _, _ = e.raw("DELETE", path+"/photos/BACK", owner); st != 204 {
		t.Fatalf("delete photo: %d", st)
	}
	if st, _, _ = e.raw("DELETE", path+"/photos/BACK", owner); st != 404 {
		t.Fatalf("delete photo twice: %d", st)
	}
	if st, _, _ = e.raw("DELETE", path+"/number", mgr); st != 204 {
		t.Fatalf("delete number: %d", st)
	}
	if st, _, _ = e.raw("POST", path+"/reveal", owner); st != 404 {
		t.Fatalf("reveal after delete: %d", st)
	}
	for action, want := range map[string]int{"GUEST_ID_REVEALED": 1, "GUEST_ID_PHOTO_VIEWED": 1, "GUEST_ID_PHOTO_DOWNLOADED": 1, "GUEST_ID_PHOTO_DELETED": 1, "GUEST_ID_NUMBER_DELETED": 1, "GUEST_ID_NUMBER_SET": 1, "GUEST_ID_PHOTO_SET": 3} {
		if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = $1 AND entity_id = $2`, action, stayID); action != "GUEST_ID_PHOTO_SET" && n != want {
			t.Errorf("%s audit rows = %d, want %d", action, n, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'GUEST_ID_REVEALED' AND actor_id = (SELECT id FROM app.users WHERE username = 'mai2')`); n != 1 {
		t.Error("the audit entry names who revealed")
	}
	// Nothing of it in audit rows, idempotency rows or logs.
	imgB64 := base64.StdEncoding.EncodeToString(img[:64])
	for _, table := range []string{"audit_logs", "idempotency_keys", "stays", "alerts"} {
		if d := e.dumpTable(table); strings.Contains(d, gidNumber) || strings.Contains(d, imgB64) || strings.Contains(d, gidMarker) {
			t.Errorf("guest ID data found in %s", table)
		}
	}
	logs := e.logs.String()
	for _, s := range []string{gidNumber, gidMarker, imgB64, string(img[:32])} {
		if strings.Contains(logs, s) {
			t.Errorf("guest ID data in the logs: %.20q", s)
		}
	}
}

// SG-805: photos can be retaken for 24 hours after check-out, not later.
func TestGuestID_EditWindow_SG805(t *testing.T) {
	e := newEnv(t)
	owner := e.ownerSetup()
	_, raw := e.checkIn(owner, 1, newKey(), stayBody(nil))
	stayID := parse(raw)["id"].(string)
	yes := true
	src := photoJPEG(t, 20, 20)
	e.exec(`UPDATE app.stays SET status = 'CHECKED_OUT', check_out_at = $2 WHERE id = $1`, stayID, e.start.Add(-time.Hour))
	if st, _, _ := e.upload(owner, stayID, "FRONT", src, "image/jpeg", &yes); st != 200 {
		t.Fatalf("one hour after check-out: %d", st)
	}
	e.exec(`UPDATE app.stays SET check_out_at = $2 WHERE id = $1`, stayID, e.start.Add(-25*time.Hour))
	if st, _, raw := e.upload(owner, stayID, "BACK", src, "image/jpeg", &yes); st != 409 || !strings.Contains(string(raw), "GUEST_ID_CLOSED") {
		t.Fatalf("25 hours after check-out: %d %s", st, raw)
	}
	if st, _ := e.send("PUT", "/v1/stays/"+stayID+"/guest-id/number", owner, newKey(), map[string]any{"idNumber": gidNumber, "consent": true}); st != 409 {
		t.Fatalf("number after the window: %d", st)
	}
}

// SG-805 AC5: the daily job deletes numbers and photos past idRetentionDays and records each deletion.
func TestGuestID_Retention_SG805_AC5(t *testing.T) {
	e := newEnv(t)
	owner := e.ownerSetupRooms(5)
	yes := true
	src := photoJPEG(t, 20, 20)
	stays := map[string]string{} // name -> stay id
	for i, name := range []string{"old", "recent", "active", "short"} {
		_, raw := e.checkIn(owner, i+1, newKey(), stayBody(map[string]any{"idNumber": gidNumber, "idConsent": true}))
		stays[name] = parse(raw)["id"].(string)
		if st, _, _ := e.upload(owner, stays[name], "FRONT", src, "image/jpeg", &yes); st != 200 {
			t.Fatalf("upload %s: %d", name, st)
		}
	}
	out := func(id string, days int) {
		e.exec(`UPDATE app.stays SET status = 'CHECKED_OUT', check_out_at = $2 WHERE id = $1`, id, e.start.Add(-time.Duration(days)*24*time.Hour))
	}
	out(stays["old"], 31)
	out(stays["recent"], 29)
	out(stays["short"], 12)
	left := func() int {
		return e.count(`SELECT count(*) FROM app.guest_ids`) + e.count(`SELECT count(*) FROM app.guest_id_photos`)
	}
	if left() != 8 {
		t.Fatalf("setup: %d rows", left())
	}
	var buf bytes.Buffer
	if err := runJobs(context.Background(), e.jobs(), []string{staffTenant}, e.start, &buf); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM app.guest_ids WHERE stay_id = $1`, stays["old"]) + e.count(`SELECT count(*) FROM app.guest_id_photos WHERE stay_id = $1`, stays["old"]); n != 0 {
		t.Fatal("the stay past the default 30 days must be purged")
	}
	if left() != 6 || strings.Contains(buf.String(), gidNumber) {
		t.Fatalf("others stay: %d rows\n%s", left(), buf.String())
	}
	if n := e.count(`SELECT count(*) FROM app.audit_logs WHERE action = 'GUEST_ID_RETENTION_DELETED' AND entity_id = $1 AND actor_id IS NULL`, stays["old"]); n != 1 {
		t.Fatal("the deletion is recorded")
	}
	// The owner shortens retention to 10 days: the 12-day-old stay now goes, the 29-day one with it.
	if r := e.patch("/v1/owner/property", owner, map[string]any{"idRetentionDays": 10}); r.status != 200 {
		t.Fatalf("property: %d %v", r.status, r.body)
	}
	if err := runJobs(context.Background(), e.jobs(), []string{staffTenant}, e.start, &buf); err != nil {
		t.Fatal(err)
	}
	if left() != 2 { // only the active stay keeps its number and photo
		t.Fatalf("after shortening retention: %d rows", left())
	}
}

func (e *env) jobs() []tenantJob {
	e.t.Helper()
	j, err := jobRegistry(config.Config{DataEncryptionKey: testDataKey}, jobDeps{uow: postgres.NewUnitOfWork(e.pool)})
	if err != nil {
		e.t.Fatal(err)
	}
	return j
}
