// The page to return to after signing in: a path inside this app and this language only. Absolute URLs,
// protocol-relative "//host", backslashes and control characters (also percent-encoded) are rejected
// (open-redirect protection).
export function safeNext(next: string | null, localeRoot: string): string | null {
  if (!next || !next.startsWith(`${localeRoot}/`)) return null;
  let decoded: string;
  try {
    decoded = decodeURIComponent(next);
  } catch {
    return null;
  }
  for (const v of [next, decoded])
    if (v.startsWith("//") || v.includes("\\") || v.includes("://") || /[\u0000-\u001f]/.test(v))
      return null;
  return next;
}
