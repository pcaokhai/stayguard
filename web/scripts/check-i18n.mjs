// Fails when messages/vi.json and messages/en.json differ in keys (a missing key breaks the build check).
import { readFileSync } from "node:fs";

const leaves = (o, p = "") =>
  Object.entries(o).flatMap(([k, v]) =>
    typeof v === "object" ? leaves(v, `${p}${k}.`) : [`${p}${k}`],
  );
const load = (l) =>
  new Set(
    leaves(JSON.parse(readFileSync(new URL(`../messages/${l}.json`, import.meta.url), "utf8"))),
  );
const vi = load("vi");
const en = load("en");
const missing = [
  ...[...vi].filter((k) => !en.has(k)).map((k) => `en missing: ${k}`),
  ...[...en].filter((k) => !vi.has(k)).map((k) => `vi missing: ${k}`),
];
if (missing.length) {
  console.error(missing.join("\n"));
  process.exit(1);
}
console.log(`i18n ok: ${vi.size} keys in vi and en`);
