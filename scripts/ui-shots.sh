#!/usr/bin/env bash
# Screenshot a route at a given size with agent-browser and print the design PNG to compare (docs/16 §6).
# Usage: scripts/ui-shots.sh <route> <BoardName> [WIDTHxHEIGHT] [BASE_URL]
#   scripts/ui-shots.sh /vi/rooms Main
#   scripts/ui-shots.sh /vi/owner TongQuanPC 1280x900 http://localhost:8080
set -euo pipefail
route="${1:?route, e.g. /vi/rooms}"
board="${2:?design board name, e.g. Main}"
size="${3:-390x844}"
base="${4:-http://localhost:3000}"
w="${size%x*}"; h="${size#*x}"
root="$(cd "$(dirname "$0")/.." && pwd)"
design="$root/docs/assets/design/screens/${board}.png"
out="$root/.shots/${board}-${w}.png"
command -v agent-browser >/dev/null || { echo "agent-browser not installed: npm i -g agent-browser && agent-browser install" >&2; exit 1; }
[ -f "$design" ] || { echo "no design PNG: $design (see docs/assets/design/INDEX.md)" >&2; exit 1; }
mkdir -p "$root/.shots"
agent-browser set viewport "$w" "$h" >/dev/null
agent-browser open "${base}${route}" >/dev/null
agent-browser wait --load networkidle >/dev/null || true
agent-browser screenshot --full "$out" >/dev/null
echo "app:    $out"
echo "design: $design"
