#!/usr/bin/env bash
# SG001_AC4: the CI secret scan (same gitleaks invocation) flags a planted fake key
# in a throwaway repo and passes on this repository's history.
set -u
cd "$(dirname "$0")/.."
repo=$PWD
command -v gitleaks >/dev/null || { echo "FAIL: gitleaks not installed" >&2; exit 1; }
scan() { (cd "$1" && gitleaks git --redact --verbose .); }

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
cd "$tmp" && git init -q . && git config user.email t@example.com && git config user.name t
# Fake, high-entropy AWS-shaped pair (not a real credential). gitleaks allowlists strings containing
# EXAMPLE, so the classic AWS docs key would be ignored.
# Assembled at runtime so this script itself never contains a scannable key in history.
id=AKIA$(printf 'Z7Q3XKJ4M2WBT5NV')
secret=$(printf 'r8Tq2LpW9xVn4KdZ7mBc1Yh''J6sUf3GaE0oNiR5tX')
printf 'aws_access_key_id = %s\naws_secret_access_key = %s\n' "$id" "$secret" > creds.txt
git add . && git commit -qm planted

if scan "$tmp" >/dev/null 2>&1; then echo "FAIL: planted secret not detected" >&2; exit 1; fi
echo "ok: planted secret detected (non-zero exit)"
scan "$repo" >/dev/null 2>&1 || { echo "FAIL: real history flagged" >&2; exit 1; }
echo "PASS test-secret-scan"
