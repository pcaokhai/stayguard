// Fails when messages/vi.json and messages/en.json differ in keys, or when code references a key that does not exist:
// t("a.b") must be a leaf in both files, and t(`a.${x}`) needs `a` to be a group with entries in both.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

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

const tree = (l) =>
  JSON.parse(readFileSync(new URL(`../messages/${l}.json`, import.meta.url), "utf8"));
const at = (o, path) =>
  path.split(".").reduce((n, k) => (n && typeof n === "object" ? n[k] : undefined), o);
const trees = { vi: tree("vi"), en: tree("en") };
const sources = (dir) =>
  readdirSync(dir).flatMap((f) => {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) return /generated|mocks/.test(f) ? [] : sources(p);
    return /\.tsx?$/.test(f) && !/\.test\./.test(f) ? [p] : [];
  });
const SRC = new URL("../src", import.meta.url).pathname;
for (const file of sources(SRC)) {
  const code = readFileSync(file, "utf8");
  const at_ = (m) => `${file.replace(SRC, "src")}: ${m}`;
  for (const m of code.matchAll(/\bt[f]?\(\s*"([A-Za-z][\w.]*)"/g))
    for (const l of ["vi", "en"])
      if (typeof at(trees[l], m[1]) !== "string") missing.push(at_(`${l} has no key ${m[1]}`));
  for (const m of code.matchAll(/\bt[f]?\(\s*`([A-Za-z][\w.]*)\$\{/g))
    for (const l of ["vi", "en"]) {
      // `a.b.${x}` needs the group a.b; `a.b_${x}` / `a.bRole${x}` needs some key in a that starts with b_ / bRole.
      const [path, stem] = m[1].endsWith(".")
        ? [m[1].slice(0, -1), ""]
        : [m[1].split(".").slice(0, -1).join("."), m[1].split(".").pop()];
      const g = path ? at(trees[l], path) : trees[l];
      const ok = g && typeof g === "object" && Object.keys(g).some((k) => k.startsWith(stem));
      if (!ok) missing.push(at_(`${l} has no keys for the dynamic key ${m[1]}\${...}`));
    }
}
if (missing.length) {
  console.error(missing.join("\n"));
  process.exit(1);
}
console.log(`i18n ok: ${vi.size} keys in vi and en`);
