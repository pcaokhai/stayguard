#!/usr/bin/env python3
"""Fills the demo guesthouse through the public API, the way staff and the bank do (nothing is written to the database
directly): PIN sign-in, check-in, extras, check-out, cash and QR payments, signed SePay webhooks, cleaning, a damage report,
expenses, a roster, a leave request and a closed shift. Called by scripts/demo-reset.sh after `tenant import`.

usage: populate.py <base-url> <import-log> <webhook-path> <sepay-secret> <account-no>
Test data only: no real people, accounts or ID numbers."""
import hashlib
import hmac
import json
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import date, datetime, timedelta, timezone

BASE, LOG, HOOK, SECRET, ACCOUNT = sys.argv[1].rstrip("/"), sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5]
CODE = "demo"
# The PINs the people of the demo use after the first sign-in (the import prints one-time PINs that must be changed).
PINS = {"owner": "482915", "mina": "739106", "linh": "260814", "viv": "731902", "hoa": "846205"}
warnings = []


def call(method, path, token=None, body=None, key=False, ok=(200, 201, 204)):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    if key:
        req.add_header("Idempotency-Key", str(uuid.uuid4()))
    try:
        with urllib.request.urlopen(req) as res:
            raw, status = res.read(), res.status
    except urllib.error.HTTPError as e:
        raw, status = e.read(), e.code
    out = json.loads(raw) if raw else {}
    if status not in ok:
        raise RuntimeError(f"{method} {path}: {status} {raw[:300]!r}")
    return out


def attempt(what, fn):
    """A scenario step that may be refused by a rule; the demo still resets, and the step is reported."""
    try:
        return fn()
    except RuntimeError as e:
        warnings.append(f"{what}: {e}")
        return None


def one_time_pins():
    pins = {}
    for line in open(LOG, encoding="utf-8"):
        parts = line.split()
        if len(parts) >= 2 and parts[0] in PINS and parts[1].isdigit():
            pins[parts[0]] = parts[1]
    missing = set(PINS) - set(pins)
    if missing:
        sys.exit(f"populate: no one-time PIN for {sorted(missing)} in the import output")
    return pins


def sign_in_all():
    tokens = {}
    for user, one_time in one_time_pins().items():
        r = call("POST", "/v1/auth/sign-in", body={"guesthouseCode": CODE, "username": user, "pin": one_time})
        call("PUT", "/v1/me/pin", r["accessToken"], {"currentPin": one_time, "newPin": PINS[user]})
        tokens[user] = call("POST", "/v1/auth/sign-in", body={"guesthouseCode": CODE, "username": user, "pin": PINS[user]})["accessToken"]
    return tokens


def rooms_by_code(token):
    out = {}
    for b in call("GET", "/v1/buildings", token)["items"]:
        for r in call("GET", f"/v1/buildings/{b['id']}/rooms", token)["items"]:
            out[r["code"]] = r["id"]
    return out


def webhook(note, amount):
    """What SePay sends: sha256= and the hex HMAC-SHA256 of "{timestamp}.{raw body}" with the tenant's secret."""
    body = json.dumps({
        "id": int(time.time() * 1000) + len(note), "gateway": "Vietcombank", "transactionDate": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "accountNumber": ACCOUNT, "subAccount": "", "code": None, "content": note, "transferType": "in", "description": "demo transfer",
        "transferAmount": amount, "accumulated": 0, "referenceCode": "FT-DEMO",
    })
    ts = str(int(time.time()))
    sig = "sha256=" + hmac.new(SECRET.encode(), f"{ts}.{body}".encode(), hashlib.sha256).hexdigest()
    req = urllib.request.Request(BASE + HOOK, data=body.encode(), method="POST", headers={
        "Content-Type": "application/json", "X-SePay-Signature": sig, "X-SePay-Timestamp": ts})
    with urllib.request.urlopen(req) as res:
        assert res.status == 200
    time.sleep(0.002)


def main():
    t = sign_in_all()
    room = rooms_by_code(t["linh"])

    def check_in(user, code, rental, guest, deposit=100000):
        return call("POST", f"/v1/rooms/{room[code]}/stays", t[user], {
            "rentalType": rental, "guestName": guest, "guestPhone": "0900000001", "deposit": deposit}, key=True)

    def extras(user, stay, items):
        call("POST", f"/v1/stays/{stay['id']}/extras", t[user], {"items": [{"serviceCode": c, "quantity": q} for c, q in items]}, key=True)

    def checkout(user, stay):
        return call("POST", f"/v1/stays/{stay['id']}/checkout", t[user], key=True)

    def pay(user, invoice, method):
        return call("POST", f"/v1/invoices/{invoice['id']}/payments", t[user], {"method": method}, key=True)

    # Building A at the front desk (Linh): rooms in use right now.
    a101 = check_in("linh", "A101", "HOURLY", "Guest A101")
    extras("linh", a101, [("WATER", 2)])
    check_in("linh", "A102", "HOURLY", "Guest A102")
    check_in("linh", "A104", "DAILY", "Guest A104", 200000)
    check_in("linh", "A105", "OVERNIGHT", "Guest A105", 150000)
    a201 = check_in("linh", "A201", "DAILY", "Guest A201", 200000)
    # An old check-in time makes the stay overdue and raises the "check-in time edited" alert for the owner.
    attempt("A201 overdue", lambda: call("POST", f"/v1/stays/{a201['id']}/check-in-time", t["linh"], {
        "newCheckInAt": (datetime.now(timezone.utc) - timedelta(hours=30)).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "reasonCode": "WRONG_TIME", "note": "Demo: guest arrived yesterday"}, key=True))

    # Paid by cash, left dirty (room waits to be cleaned).
    s = check_in("linh", "A103", "HOURLY", "Guest A103")
    extras("linh", s, [("SODA", 1)])
    pay("linh", checkout("linh", s), "CASH")

    # Paid by QR: the bank's signed webhook settles it; Hoa cleans the room afterwards.
    s = check_in("linh", "A302", "OVERNIGHT", "Guest A302", 150000)
    extras("linh", s, [("BEER", 2), ("NOODLE", 1)])
    inv = checkout("linh", s)
    p = pay("linh", inv, "TRANSFER")
    webhook(f"{inv['billCode']} chuyen tien", p["amount"])

    # QR shown, money not received yet.
    s = check_in("linh", "A106", "HOURLY", "Guest A106", 10000)
    pay("linh", checkout("linh", s), "TRANSFER")

    # Underpaid by the bank: the invoice stays open with the remainder.
    s = check_in("linh", "A202", "OVERNIGHT", "Guest A202", 100000)
    inv = checkout("linh", s)
    p = pay("linh", inv, "TRANSFER")
    webhook(f"{inv['billCode']} chuyen tien", max(p["amount"] - 50000, 10000))

    # Overpaid by the bank.
    s = check_in("linh", "A203", "HOURLY", "Guest A203", 10000)
    inv = checkout("linh", s)
    p = pay("linh", inv, "TRANSFER")
    webhook(f"{inv['billCode']} chuyen tien", p["amount"] + 30000)

    # Money with no bill code: the owner sees an unmatched transfer.
    webhook("nap tien khong ro noi dung", 150000)

    # Building B (Viv).
    for code, rental, dep in (("B102", "HOURLY", 100000), ("B106", "DAILY", 200000), ("B203", "OVERNIGHT", 150000), ("B302", "HOURLY", 100000)):
        check_in("viv", code, rental, "Guest " + code, dep)
    s = check_in("viv", "B104", "HOURLY", "Guest B104")
    extras("viv", s, [("WATER", 3), ("TOWEL", 1)])
    pay("viv", checkout("viv", s), "CASH")

    # Housekeeping: Hoa cleans one room; the rest stay in the list.
    tasks = call("GET", "/v1/housekeeping/tasks", t["hoa"])["items"]
    for task in tasks:
        if task["roomCode"] == "A302":
            call("POST", f"/v1/housekeeping/tasks/{task['id']}/complete", t["hoa"], key=True)

    # A damage report locks a room and opens a repair ticket; the owner prices it.
    attempt("damage report", lambda: call("POST", f"/v1/rooms/{room['A205']}/damage-reports", t["hoa"], {
        "category": "AIR_CONDITIONER", "severity": "LOCK_ROOM", "description": "Demo: air conditioner does not cool"}, key=True))
    for ticket in call("GET", "/v1/owner/maintenance-tickets", t["owner"])["items"]:
        attempt("ticket", lambda tk=ticket: call("PATCH", f"/v1/owner/maintenance-tickets/{tk['id']}", t["owner"], {
            "status": "IN_REPAIR", "repairer": "Demo Cool Co.", "partsCost": 350000, "labourCost": 150000}))

    # Owner books: expenses of the month, a roster for the week, a leave request.
    month = date.today().strftime("%Y-%m")
    for category, amount, recurring in (("RENT", 12000000, True), ("ELECTRICITY", 2400000, False), ("WATER", 600000, False), ("SUPPLIES", 450000, False)):
        call("POST", "/v1/owner/expenses", t["owner"], {"category": category, "amount": amount, "month": month, "paidOn": date.today().isoformat(),
                                                          "recurring": recurring}, key=True)
    staff = {s["username"]: s["id"] for s in call("GET", "/v1/owner/staff", t["owner"])["items"]}
    monday = date.today() - timedelta(days=date.today().weekday())
    shifts = [{"userId": staff[u], "date": (monday + timedelta(days=d)).isoformat(), "shift": sh}
              for d in range(7) for u, sh in (("linh", "MORNING"), ("viv", "AFTERNOON"), ("hoa", "MORNING"), ("mina", "NIGHT"))]
    attempt("roster", lambda: call("PUT", "/v1/owner/roster", t["owner"], {"set": shifts, "remove": []}, key=True))
    nxt = monday + timedelta(days=9)
    attempt("leave request", lambda: call("POST", "/v1/me/leave-requests", t["linh"], {
        "fromDate": nxt.isoformat(), "toDate": nxt.isoformat(), "kind": "PAID", "reason": "Demo: family event"}, key=True))

    # Viv closes the shift: counted cash a little short of expected, so the owner has a difference to review.
    cur = call("GET", "/v1/shifts/current", t["viv"])
    counted, counts = max(cur["expectedCash"] - 10000, 0), []
    for denom in (500000, 200000, 100000, 50000, 20000, 10000):
        n, counted = divmod(counted, denom)
        counts.append({"denomination": denom, "quantity": n})
    attempt("close shift", lambda: call("POST", "/v1/shifts/current/close", t["viv"], {
        "counts": counts, "floatLeft": 0, "reason": "Demo: 10.000 short, small change not counted"}, key=True))

    for w in warnings:
        print("warning:", w, file=sys.stderr)
    print("populated")


main()
