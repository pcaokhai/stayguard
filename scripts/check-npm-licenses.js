// Fails if any production npm dependency (incl. transitive) has a licence outside the allowlist.
// Reads installed package.json files; no extra tool. Run from web/ after `npm ci`.
const { execSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const ALLOWED = new Set(["Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "0BSD"]);
// --parseable prints one install path per line (root first); dedupe via Set.
const dirs = new Set(execSync("npm ls --omit=dev --all --parseable", { maxBuffer: 1 << 26 }).toString().split("\n").filter(Boolean).slice(1));

// Reviewed exceptions (owner decision pending, see SG-001 task-C report): sharp is Next's optional image
// optimiser (LGPL libvips binaries, unused by a static export); caniuse-lite is CC-BY-4.0 browser data.
// Matched against the full package name (scope included), not the folder basename.
const EXCEPT = /^(@img\/sharp-(libvips-.+|wasm32)|caniuse-lite)$/;

// An SPDX "OR" expression is fine when any alternative is allowed.
const ok = (l) => l.replace(/[()]/g, "").split(/\s+OR\s+/).some((x) => ALLOWED.has(x.trim()));
let bad = 0;
for (const dir of dirs) {
  const pj = JSON.parse(fs.readFileSync(path.join(dir, "package.json"), "utf8"));
  const name = pj.name;
  const lic = typeof pj.license === "string" ? pj.license : (pj.license && pj.license.type) || "UNKNOWN";
  if (!ok(lic) && !EXCEPT.test(name)) { console.error(`${name}@${pj.version}: ${lic}`); bad = 1; }
}
console.log(`checked ${dirs.size} production packages`);
process.exit(bad);
