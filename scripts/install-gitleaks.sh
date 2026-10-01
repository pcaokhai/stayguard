#!/usr/bin/env bash
# Downloads the pinned gitleaks into ./gitleaks and verifies its sha256 (used by CI jobs).
# Needs GITLEAKS_VERSION and GITLEAKS_SHA256 (linux x64 tarball) in the environment.
set -euo pipefail
f="gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz"
curl -fsSL -o "$f" "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/$f"
echo "${GITLEAKS_SHA256}  $f" | sha256sum -c -
tar -xzf "$f" gitleaks
