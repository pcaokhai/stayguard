// orval mocks a binary response (getGuestIdPhoto) as a string, which does not type-check as Blob.
import { readFileSync, writeFileSync } from "node:fs";

const file = "src/mocks/generated/api.msw.ts";
const src = readFileSync(file, "utf8");
const fixed = src.replace(
  /(\(\): Blob => )\(faker\.string\.alpha\([^)]*\{[^}]*\}\}?\)\)/g,
  '$1new Blob(["mock"], { type: "image/jpeg" })',
);
writeFileSync(file, fixed);
