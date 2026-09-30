#!/usr/bin/env bash
# SG001_AC6: .claude/settings.json parses, and every build/vendored directory listed
# in .gitignore (lines ending in "/") has a matching Read deny rule.
set -eu
cd "$(dirname "$0")/.."
node -e '
const fs = require("fs");
const s = JSON.parse(fs.readFileSync(".claude/settings.json", "utf8"));
const deny = ((s.permissions || {}).deny || []).filter((r) => r.startsWith("Read("));
const dirs = fs.readFileSync(".gitignore", "utf8").split("\n").filter((l) => /^[^#!].*\/$/.test(l));
const missing = dirs.filter((d) => !deny.some((r) => r.includes(d.replace(/\/$/, "/"))));
if (missing.length) { console.error("FAIL: no Read deny for: " + missing.join(", ")); process.exit(1); }
console.log("PASS check-claude-deny (" + dirs.length + " dirs, " + deny.length + " Read denies)");
'
