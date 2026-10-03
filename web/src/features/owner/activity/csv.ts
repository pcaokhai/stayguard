import { csvLine, type AuditEntry } from "./text";

// Exports the rows already loaded on screen (the log stays append-only on the server).
export function downloadCsv(entries: AuditEntry[], filename: string) {
  const lines = entries.map(csvLine);
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), { href: url, download: filename });
  a.click();
  URL.revokeObjectURL(url);
}
