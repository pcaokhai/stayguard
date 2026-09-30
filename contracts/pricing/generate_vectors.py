#!/usr/bin/env python3
"""Reference implementation of the StayGuard pricing rules (docs/02 section 7.7, docs/05 section 3).

Purpose: generate contracts/pricing/golden-cases.json. It is NOT product code; the Go implementation
in api/ must reproduce every case. Rule change => edit this file, regenerate, review the diff.
Usage: python contracts/pricing/generate_vectors.py [--check]
"""
import json
import sys
from datetime import datetime, timedelta, time
from pathlib import Path
from zoneinfo import ZoneInfo

TZ = ZoneInfo("Asia/Ho_Chi_Minh")
GRACE = 15
PLANS = {
    "STANDARD": {"firstHour": 80000, "extraHour": 20000, "overnight": 200000, "daily": 300000},
    "VIP": {"firstHour": 120000, "extraHour": 30000, "overnight": 300000, "daily": 450000},
}
OUT = Path(__file__).with_name("golden-cases.json")


class PricingError(Exception):
    def __init__(self, code):
        self.code = code


def at(s):
    return datetime.fromisoformat(s).astimezone(TZ)


def minutes_between(a, b):
    return int((b - a).total_seconds() // 60)


def blocks(minutes):
    full, rem = divmod(minutes, 60)
    return full + (1 if rem > GRACE else 0)


def line(code, qty, unit):
    return {"code": code, "quantity": qty, "unitAmount": unit, "amount": qty * unit}


def at_time(day, hh, mm=0):
    return datetime.combine(day, time(hh, mm), tzinfo=TZ)


def price(plan_name, rental, cin, cout):
    p, t, o = PLANS[plan_name], at(cin), at(cout)
    if o <= t:
        raise PricingError("PRICING_INVALID_INTERVAL")
    mins = minutes_between(t, o)
    lines, capped = [], False
    if rental == "HOURLY":
        lines.append(line("FIRST_HOUR", 1, p["firstHour"]))
        if mins > 60 and blocks(mins - 60):
            lines.append(line("EXTRA_HOUR", blocks(mins - 60), p["extraHour"]))
    elif rental == "OVERNIGHT":
        if t.time() >= time(21, 0):
            d = t.date()
        elif t.time() < time(12, 0):
            d = t.date() - timedelta(days=1)
        else:
            d = t.date()
        start, end = at_time(d, 21), at_time(d + timedelta(days=1), 12)
        lines.append(line("OVERNIGHT", 1, p["overnight"]))
        early, late = blocks(max(0, minutes_between(t, start))), blocks(max(0, minutes_between(end, o)))
        if early:
            lines.append(line("EARLY_CHECKIN_HOUR", early, p["extraHour"]))
        if late:
            lines.append(line("LATE_CHECKOUT_HOUR", late, p["extraHour"]))
    elif rental == "DAILY":
        d0 = t.date()
        early_fee = min(blocks(max(0, minutes_between(t, at_time(d0, 14)))) * p["extraHour"], p["daily"])
        i = 1
        while o > at_time(d0 + timedelta(days=i), 12):
            i += 1
        days, late_fee = 1, 0
        if i > 1:
            days = i - 1
            late_fee = min(blocks(minutes_between(at_time(d0 + timedelta(days=i - 1), 12), o)) * p["extraHour"], p["daily"])
            if late_fee >= p["daily"]:
                days, late_fee = days + 1, 0
        lines.append(line("DAILY", days, p["daily"]))
        if early_fee:
            lines.append(line("EARLY_CHECKIN_FEE", 1, early_fee))
        if late_fee:
            lines.append(line("LATE_CHECKOUT_FEE", 1, late_fee))
    else:
        raise PricingError("PRICING_UNKNOWN_RENTAL_TYPE")
    total = sum(x["amount"] for x in lines)
    if rental in ("HOURLY", "OVERNIGHT") and total > p["daily"]:
        capped, total = True, p["daily"]
    return {"total": total, "capped": capped, "lines": lines}


CASES = [
    ("PRC-H01", "45-minute stay pays the first hour", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T10:45:00+07:00", 80000),
    ("PRC-H02", "exactly 60 minutes", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T11:00:00+07:00", 80000),
    ("PRC-H03", "1h10m is inside the grace period", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T11:10:00+07:00", 80000),
    ("PRC-H04", "1h15m is the grace boundary", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T11:15:00+07:00", 80000),
    ("PRC-H05", "1h16m passes the grace boundary", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T11:16:00+07:00", 100000),
    ("PRC-H06", "1h20m", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T11:20:00+07:00", 100000),
    ("PRC-H07", "exactly 2h", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T12:00:00+07:00", 100000),
    ("PRC-H08", "2h35m", "STANDARD", "HOURLY", "2026-10-05T11:25:00+07:00", "2026-10-05T14:00:00+07:00", 120000),
    ("PRC-H09", "14 hours hits the daily cap", "STANDARD", "HOURLY", "2026-10-05T08:00:00+07:00", "2026-10-05T22:00:00+07:00", 300000),
    ("PRC-H10", "VIP 2h35m", "VIP", "HOURLY", "2026-10-05T11:25:00+07:00", "2026-10-05T14:00:00+07:00", 180000),
    ("PRC-O01", "regular night", "STANDARD", "OVERNIGHT", "2026-10-05T22:00:00+07:00", "2026-10-06T10:00:00+07:00", 200000),
    ("PRC-O02", "1h30m early check-in", "STANDARD", "OVERNIGHT", "2026-10-05T19:30:00+07:00", "2026-10-06T10:00:00+07:00", 240000),
    ("PRC-O03", "1h10m late check-out", "STANDARD", "OVERNIGHT", "2026-10-05T22:00:00+07:00", "2026-10-06T13:10:00+07:00", 220000),
    ("PRC-O04", "check-in after midnight belongs to the previous night", "STANDARD", "OVERNIGHT", "2026-10-06T01:00:00+07:00", "2026-10-06T12:00:00+07:00", 200000),
    ("PRC-O05", "midday check-in is capped at the daily price", "STANDARD", "OVERNIGHT", "2026-10-05T12:30:00+07:00", "2026-10-06T12:00:00+07:00", 300000),
    ("PRC-O06", "late check-out exactly at grace", "STANDARD", "OVERNIGHT", "2026-10-05T22:00:00+07:00", "2026-10-06T12:15:00+07:00", 200000),
    ("PRC-O07", "late check-out one minute past grace", "STANDARD", "OVERNIGHT", "2026-10-05T22:00:00+07:00", "2026-10-06T12:16:00+07:00", 220000),
    ("PRC-O08", "10 minutes early is inside grace", "STANDARD", "OVERNIGHT", "2026-10-05T20:50:00+07:00", "2026-10-06T10:00:00+07:00", 200000),
    ("PRC-D01", "one day, on time", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-06T12:00:00+07:00", 300000),
    ("PRC-D02", "two days, on time", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-07T12:00:00+07:00", 600000),
    ("PRC-D03", "late check-out 1h10m", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-06T13:10:00+07:00", 320000),
    ("PRC-D04", "1h early check-in", "STANDARD", "DAILY", "2026-10-05T13:00:00+07:00", "2026-10-06T12:00:00+07:00", 320000),
    ("PRC-D05", "late check-out 2h", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-06T14:00:00+07:00", 340000),
    ("PRC-D06", "checking out just before the second noon still pays two days", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-07T11:59:00+07:00", 600000),
    ("PRC-D07", "VIP two days", "VIP", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-07T12:00:00+07:00", 900000),
]
ERRORS = [
    ("PRC-E01", "check-out equals check-in", "STANDARD", "HOURLY", "2026-10-05T10:00:00+07:00", "2026-10-05T10:00:00+07:00", "PRICING_INVALID_INTERVAL"),
    ("PRC-E02", "check-out before check-in", "STANDARD", "DAILY", "2026-10-05T14:00:00+07:00", "2026-10-05T09:00:00+07:00", "PRICING_INVALID_INTERVAL"),
]
BILLS = [
    {"id": "PRC-B01", "name": "2h35m, two waters, 100,000 deposit: balance due 40,000", "caseRef": "PRC-H08",
     "extras": [{"serviceCode": "WATER", "quantity": 2, "unitAmount": 10000}], "deposit": 100000},
    {"id": "PRC-B02", "name": "deposit larger than the bill: refund", "caseRef": "PRC-H01",
     "extras": [], "deposit": 100000},
]


def build():
    cases = []
    for cid, name, plan, rental, cin, cout, want in CASES:
        got = price(plan, rental, cin, cout)
        assert got["total"] == want, f"{cid}: expected {want}, reference gave {got['total']}"
        cases.append({"id": cid, "name": name, "roomType": plan, "rentalType": rental, "checkIn": cin, "checkOut": cout, "expected": got})
    errors = []
    for cid, name, plan, rental, cin, cout, code in ERRORS:
        try:
            price(plan, rental, cin, cout)
        except PricingError as e:
            assert e.code == code
        else:
            raise AssertionError(f"{cid}: expected error")
        errors.append({"id": cid, "name": name, "roomType": plan, "rentalType": rental, "checkIn": cin, "checkOut": cout, "expectedError": code})
    by_id = {c["id"]: c for c in cases}
    bills = []
    for b in BILLS:
        stay = by_id[b["caseRef"]]["expected"]["total"]
        extras = sum(e["quantity"] * e["unitAmount"] for e in b["extras"])
        total = stay + extras
        bills.append({**b, "expected": {"stayTotal": stay, "extrasTotal": extras, "total": total,
                                        "balanceDue": max(0, total - b["deposit"]), "refundDue": max(0, b["deposit"] - total)}})
    assert bills[0]["expected"]["balanceDue"] == 40000 and bills[1]["expected"]["refundDue"] == 20000
    return {"$comment": "GENERATED by generate_vectors.py. Do not edit by hand.", "timezone": "Asia/Ho_Chi_Minh",
            "graceMinutes": GRACE, "currency": "VND", "ratePlans": PLANS, "cases": cases, "errorCases": errors, "billCases": bills}


if __name__ == "__main__":
    text = json.dumps(build(), indent=2, ensure_ascii=False) + "\n"
    if "--check" in sys.argv:
        sys.exit(0 if OUT.read_text() == text else 1)
    OUT.write_text(text)
    print(f"wrote {OUT} ({len(CASES)} cases, {len(ERRORS)} error cases, {len(BILLS)} bill cases)")
