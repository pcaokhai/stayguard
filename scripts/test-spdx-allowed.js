// The licence check reads SPDX expressions: OR needs one allowed alternative, AND needs every part allowed, and nothing outside the allowlist passes.
const assert = require("node:assert");
const { isAllowed } = require("./spdx-allowed.js");
const ALLOWED = new Set(["Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MPL-2.0", "0BSD"]);
const cases = [
  ["MIT", true],
  ["MIT AND ISC", true], // victory-vendor@37.3.6
  ["(MIT AND ISC)", true],
  ["MIT OR GPL-3.0", true],
  ["GPL-3.0 OR MIT", true],
  ["MIT AND GPL-3.0", false], // both are required
  ["(MIT OR Apache-2.0) AND ISC", true],
  ["(MIT OR Apache-2.0) AND GPL-3.0", false],
  ["GPL-3.0", false],
  ["GPL-3.0 OR LGPL-2.1", false],
  ["UNKNOWN", false],
  ["", false],
];
for (const [expr, want] of cases) assert.strictEqual(isAllowed(expr, ALLOWED), want, expr);
console.log("PASS test-spdx-allowed (" + cases.length + " cases)");
