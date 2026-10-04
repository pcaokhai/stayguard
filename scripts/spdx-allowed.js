// Is an SPDX licence expression allowed? OR needs one allowed alternative, AND needs every part allowed (AND binds tighter than OR),
// parentheses group. An unknown or empty expression is not allowed.
function isAllowed(expr, allowed) {
  const tokens = String(expr || "").match(/\(|\)|[^\s()]+/g) || [];
  let i = 0;
  const parseOr = () => {
    let ok = parseAnd();
    while (tokens[i] === "OR") { i++; const r = parseAnd(); ok = ok || r; }
    return ok;
  };
  const parseAnd = () => {
    let ok = parseTerm();
    while (tokens[i] === "AND") { i++; const r = parseTerm(); ok = ok && r; }
    return ok;
  };
  const parseTerm = () => {
    const t = tokens[i++];
    if (t === "(") { const ok = parseOr(); if (tokens[i] === ")") i++; return ok; }
    return t !== undefined && t !== ")" && allowed.has(t.replace(/\+$/, ""));
  };
  if (tokens.length === 0) return false;
  const ok = parseOr();
  return ok && i >= tokens.length;
}
module.exports = { isAllowed };
