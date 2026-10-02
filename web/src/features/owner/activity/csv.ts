import { actionText, type AuditEntry } from "./text";

const cell = (v: string) => `"${v.replace(/"/g, '""')}"`;

// Exports the rows already loaded on screen (the log stays append-only on the server).
export function downloadCsv(entries: AuditEntry[], filename: string) {
  const lines = entries.map((e) =>
    [e.at, e.actorName, e.category, actionText(e)].map(cell).join(","),
  );
  const blob = new Blob(["﻿" + lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = Object.assign(document.createElement("a"), { href: url, download: filename });
  a.click();
  URL.revokeObjectURL(url);
}
