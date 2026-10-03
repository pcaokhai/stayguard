#!/usr/bin/env python3
"""Prints a session token for one rehearsal user (for rehearsal-shots.sh). Uses the PIN the specs chose, else the installer's one-time
PIN, then chooses 482916 like the specs do. Reads RH_GUESTHOUSE, E2E_BASE_URL and RH_PINS_FILE; never prints a PIN.
usage: rehearsal-session.py <username>"""
import json, os, sys, urllib.request

NEW_PIN = "482916"
base, code, pins_file = os.environ["E2E_BASE_URL"], os.environ["RH_GUESTHOUSE"], os.environ["RH_PINS_FILE"]
user = sys.argv[1]


def load(path):
    try:
        return json.load(open(path))
    except FileNotFoundError:
        return {}


def call(method, path, body=None, token=None):
    req = urllib.request.Request(base + path, method=method, data=json.dumps(body).encode() if body is not None else None)
    req.add_header("content-type", "application/json")
    if token:
        req.add_header("authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            return r.status, json.loads(r.read() or "null")
    except urllib.error.HTTPError as e:
        return e.code, None


chosen, one = load(pins_file + ".set").get(user), load(pins_file).get(user)
status, body = call("POST", "/v1/auth/sign-in", {"guesthouseCode": code, "username": user, "pin": chosen or one})
if status != 200:
    sys.exit(f"sign-in as {user} failed ({status})")
token = body["accessToken"]
if body.get("mustChangePin"):
    if call("PUT", "/v1/me/pin", {"currentPin": one, "newPin": NEW_PIN}, token)[0] != 204:
        sys.exit(f"could not set a PIN for {user}")
    sets = load(pins_file + ".set")
    sets[user] = NEW_PIN
    json.dump(sets, open(pins_file + ".set", "w"))
print(token)
