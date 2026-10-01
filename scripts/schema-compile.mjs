// Compile every contracts/**/*.schema.json with Ajv (JSON Schema 2020-12, strict).
// Usage: node scripts/schema-compile.mjs <contracts-dir>. Exit 1 on any compile error.
import { createRequire } from 'node:module';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';

// Ajv is installed in scripts/node_modules; the directory under test may be a temporary copy.
const require = createRequire(import.meta.url);
const Ajv2020 = require('ajv/dist/2020');
const addFormats = require('ajv-formats');

const dir = resolve(process.argv[2] ?? 'contracts');
const files = [];
const walk = (d) => {
  for (const name of readdirSync(d)) {
    if (name === 'node_modules') continue;
    const p = join(d, name);
    if (statSync(p).isDirectory()) walk(p);
    else if (name.endsWith('.schema.json')) files.push(p);
  }
};
walk(dir);
if (files.length === 0) {
  console.error(`no *.schema.json found under ${dir}`);
  process.exit(1);
}

let failed = 0;
for (const f of files) {
  try {
    const ajv = new Ajv2020({ strict: true, allErrors: true });
    addFormats(ajv);
    ajv.compile(JSON.parse(readFileSync(f, 'utf8')));
    console.log(`compiled ${f}`);
  } catch (e) {
    failed += 1;
    console.error(`${f}: ${e.message}`);
  }
}
process.exit(failed === 0 ? 0 : 1);
