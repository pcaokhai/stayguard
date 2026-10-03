#!/usr/bin/env python3
"""Writes index.html, a contact sheet of the screenshots under a shots-<date> folder: one block per role and screen, vi and en side by
side, three widths each; click a picture for the full size. usage: rehearsal-index.py <shots dir>"""
import collections, html, pathlib, re, sys

root = pathlib.Path(sys.argv[1])
rows = collections.defaultdict(lambda: collections.defaultdict(dict))  # (role, screen) -> locale -> width -> path
for p in sorted(root.glob("*/*/*.png")):
    role, locale = p.parts[-3], p.parts[-2]
    m = re.match(r"(.+)-(\d+)\.png$", p.name)
    if m:
        rows[(role, m.group(1))][locale][int(m.group(2))] = p.relative_to(root).as_posix()
out = ["<!doctype html><meta charset=utf-8><meta name=viewport content='width=device-width,initial-scale=1'><title>Rehearsal shots</title>",
       "<style>body{font:14px system-ui;margin:16px;background:#f4f3ee;color:#1d1d1b}h2{margin:28px 0 4px}h3{margin:14px 0 4px;font-weight:600}",
       ".row{display:flex;gap:8px;align-items:flex-start;overflow-x:auto;padding-bottom:8px}.row a{flex:none}",
       "img{display:block;border:1px solid #ccc;background:#fff;height:360px;width:auto}.cap{font-size:11px;color:#666}</style>",
       "<h1>Rehearsal screens</h1><p>vi then en; 390, 834 and 1280 px. No assertions: look for overflow, clipped text, wrong language.</p>"]
last = None
for (role, screen), by in sorted(rows.items()):
    if role != last:
        out.append(f"<h2>{html.escape(role)}</h2>")
        last = role
    out.append(f"<h3>{html.escape(screen.replace('_', '/'))}</h3><div class=row>")
    for locale in ("vi", "en"):
        for w in sorted(by.get(locale, {})):
            src = by[locale][w]
            out.append(f"<a href='{src}'><img loading=lazy src='{src}' alt=''><div class=cap>{locale} {w}</div></a>")
    out.append("</div>")
(root / "index.html").write_text("\n".join(out))
print(f"index: {len(rows)} screens")
