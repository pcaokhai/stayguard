#!/usr/bin/env python3
"""Fills the Status, Date and Notes columns of docs/rehearsal/checklist.xlsx from a rehearsal results CSV, for the rows whose ID matches.
It never touches a row marked manual (any cell reading "manual"), never overwrites a formula, and edits the sheet XML in place, so
styles, formulas, validation and everything else in the workbook stay as Khai saved them. No third-party packages.

usage: scripts/rehearsal-sync.py [results.csv] [--xlsx docs/rehearsal/checklist.xlsx] [--date YYYY-MM-DD] [--map pass=Pass,fail=Fail,skip=Skip]
       scripts/rehearsal-sync.py --selftest
Without a CSV it uses the newest docs/rehearsal/results-*.csv. The sheet needs a header row with ID, Status, Date and Notes cells."""
import argparse, csv, glob, os, re, shutil, sys, tempfile, zipfile
from xml.sax.saxutils import escape, unescape

CELL = re.compile(r'<c r="([A-Z]+)(\d+)"([^>]*?)(?:/>|>(.*?)</c>)', re.S)
ROW = re.compile(r'(<row r="(\d+)"[^>]*?)(?:/>|>(.*?)</row>)', re.S)
HEADERS = {"id": {"id", "ma", "mã"}, "status": {"status", "trang thai", "trạng thái"}, "date": {"date", "ngay", "ngày"},
           "notes": {"notes", "note", "ghi chu", "ghi chú"}}
MANUAL = {"manual", "thu cong", "thủ công"}


def col_index(letters):
    n = 0
    for ch in letters:
        n = n * 26 + ord(ch) - 64
    return n


def shared_strings(z):
    if "xl/sharedStrings.xml" not in z.namelist():
        return []
    xml = z.read("xl/sharedStrings.xml").decode("utf-8")
    return [unescape("".join(re.findall(r"<t[^>]*>(.*?)</t>", si, re.S))) for si in re.findall(r"<si>(.*?)</si>", xml, re.S)]


def cell_text(attrs, inner, sst):
    if inner is None:
        return ""
    kind = re.search(r'\bt="(\w+)"', attrs)
    kind = kind.group(1) if kind else ""
    if kind == "inlineStr":
        return unescape("".join(re.findall(r"<t[^>]*>(.*?)</t>", inner, re.S)))
    v = re.search(r"<v>(.*?)</v>", inner, re.S)
    if not v:
        return ""
    return sst[int(v.group(1))] if kind == "s" else unescape(v.group(1))


def sheet_files(z):
    wb = z.read("xl/workbook.xml").decode("utf-8")
    rels = z.read("xl/_rels/workbook.xml.rels").decode("utf-8")
    target = {m.group(1): m.group(2) for m in re.finditer(r'<Relationship [^>]*?Id="([^"]+)"[^>]*?Target="([^"]+)"', rels)}
    target.update({m.group(2): m.group(1) for m in re.finditer(r'<Relationship [^>]*?Target="([^"]+)"[^>]*?Id="([^"]+)"', rels)})
    out = []
    for m in re.finditer(r'<sheet [^>]*?name="([^"]*)"[^>]*?r:id="([^"]+)"', wb):
        t = target[m.group(2)].lstrip("/")
        out.append((unescape(m.group(1)), t if t.startswith("xl/") else "xl/" + t))
    return out


def inline_cell(ref, style, text):
    return f'<c r="{ref}"{style} t="inlineStr"><is><t xml:space="preserve">{escape(text)}</t></is></c>'


def set_cell(row_xml, letters, rownum, text):
    """Returns (new row xml, written?). A formula cell is left alone."""
    ref = f"{letters}{rownum}"
    for m in CELL.finditer(row_xml):
        if m.group(1) == letters:
            if m.group(4) and "<f" in m.group(4):
                return row_xml, False
            style = re.search(r'\bs="\d+"', m.group(3))
            new = inline_cell(ref, (" " + style.group(0)) if style else "", text)
            return row_xml[: m.start()] + new + row_xml[m.end():], True
    new = inline_cell(ref, "", text)
    for m in CELL.finditer(row_xml):  # keep columns in order
        if col_index(m.group(1)) > col_index(letters):
            return row_xml[: m.start()] + new + row_xml[m.start():], True
    return row_xml.replace("</row>", new + "</row>"), True


def sync(xlsx, results, date, status_map):
    z = zipfile.ZipFile(xlsx)
    sst = shared_strings(z)
    changed, report = {}, []
    for name, path in sheet_files(z):
        xml = z.read(path).decode("utf-8")
        cols, header_row = {}, 0
        for rm in ROW.finditer(xml):  # the header row: ID and Status cells
            texts = {m.group(1): cell_text(m.group(3), m.group(4), sst).strip().lower() for m in CELL.finditer(rm.group(0))}
            inv = {}
            for c, t in texts.items():
                for k, names in HEADERS.items():
                    if t in names:
                        inv[k] = c
            if "id" in inv and "status" in inv:
                cols, header_row = inv, int(rm.group(2))
                break
        if not cols:
            continue

        def edit(rm):
            head, rownum, body = rm.group(1), int(rm.group(2)), rm.group(3)
            if rownum <= header_row or body is None:
                return rm.group(0)
            row = f"{head}>{body}</row>"
            cells = {m.group(1): cell_text(m.group(3), m.group(4), sst).strip() for m in CELL.finditer(row)}
            rid = cells.get(cols["id"], "")
            if rid not in results:
                return rm.group(0)
            if any(t.lower() in MANUAL for t in cells.values()):
                report.append(f"{name}!{rid}: manual row, left alone")
                return rm.group(0)
            r, wrote = results[rid], []
            values = {"status": status_map[r["status"]], "date": date, "notes": r["notes"] or r["evidence"]}
            for key, letters in cols.items():
                if key in values:
                    row, ok = set_cell(row, letters, rownum, values[key])
                    wrote.append(key if ok else f"{key} (formula, kept)")
            report.append(f"{name}!{rid}: {r['status']} -> " + ", ".join(wrote))
            return row

        new = ROW.sub(edit, xml)
        if new != xml:
            changed[path] = new
    if not changed:
        return report
    tmp = xlsx + ".tmp"
    with zipfile.ZipFile(xlsx) as src, zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as dst:
        for item in src.infolist():
            data = src.read(item.filename)
            if item.filename in changed:
                data = changed[item.filename].encode("utf-8")
            dst.writestr(item, data)
    os.replace(tmp, xlsx)
    return report


def read_results(path):
    with open(path, newline="", encoding="utf-8") as f:
        return {r["id"]: r for r in csv.DictReader(f)}


def selftest():
    d = tempfile.mkdtemp()
    x = os.path.join(d, "c.xlsx")
    sheet = (
        '<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>'
        '<row r="1"><c r="A1" t="inlineStr"><is><t>ID</t></is></c><c r="B1" t="inlineStr"><is><t>Title</t></is></c><c r="C1" t="inlineStr"><is><t>Status</t></is></c>'
        '<c r="D1" t="inlineStr"><is><t>Date</t></is></c><c r="E1" t="inlineStr"><is><t>Notes</t></is></c></row>'
        '<row r="2"><c r="A2" t="inlineStr"><is><t>TT-01</t></is></c><c r="C2" s="3"/></row>'
        '<row r="3"><c r="A3" t="inlineStr"><is><t>TT-05</t></is></c><c r="B3" t="inlineStr"><is><t>manual</t></is></c><c r="C3" t="inlineStr"><is><t>keep</t></is></c></row>'
        '<row r="4"><c r="A4" t="inlineStr"><is><t>TT-02</t></is></c><c r="C4"><f>1+1</f><v>2</v></c><c r="E4" t="inlineStr"><is><t>old</t></is></c></row>'
        '<row r="5"><c r="A5" t="inlineStr"><is><t>ZZ-99</t></is></c></row></sheetData></worksheet>'
    )
    with zipfile.ZipFile(x, "w") as z:
        z.writestr("[Content_Types].xml", '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>')
        z.writestr("xl/workbook.xml", '<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Checklist" sheetId="1" r:id="rId1"/></sheets></workbook>')
        z.writestr("xl/_rels/workbook.xml.rels", '<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>')
        z.writestr("xl/worksheets/sheet1.xml", sheet)
    res = {k: {"id": k, "status": s, "notes": n, "evidence": "ev/" + k} for k, s, n in [("TT-01", "pass", ""), ("TT-02", "fail", "boom"), ("TT-05", "pass", "")]}
    sync(x, res, "2026-10-03", {"pass": "Pass", "fail": "Fail", "skip": "Skip"})
    out = zipfile.ZipFile(x).read("xl/worksheets/sheet1.xml").decode()
    assert '<c r="C2" s="3" t="inlineStr"><is><t xml:space="preserve">Pass</t></is></c>' in out, out  # style kept, status written
    assert '<c r="D2" t="inlineStr"><is><t xml:space="preserve">2026-10-03</t></is></c>' in out, out  # missing cell inserted in order
    assert "<f>1+1</f>" in out and ">Fail<" not in out.split("<row r=\"4\"")[1].split("</c>")[0], out  # the formula stays; the others are written
    assert ">boom<" in out and ">old<" not in out, out  # notes replaced
    assert ">keep<" in out and out.count("Pass") == 1, out  # the manual row untouched
    shutil.rmtree(d)
    print("selftest ok")


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        selftest()
        sys.exit(0)
    ap = argparse.ArgumentParser()
    ap.add_argument("csv", nargs="?")
    ap.add_argument("--xlsx", default="docs/rehearsal/checklist.xlsx")
    ap.add_argument("--date")
    ap.add_argument("--map", default="pass=Pass,fail=Fail,skip=Skip")
    a = ap.parse_args()
    path = a.csv or max(glob.glob("docs/rehearsal/results-*.csv"), default=None)
    if not path or not os.path.exists(a.xlsx):
        sys.exit(f"need a results CSV and {a.xlsx} (Khai commits the checklist there first)")
    day = a.date or (re.search(r"results-(\d{4}-\d{2}-\d{2})", path) or [None, ""])[1]
    smap = dict(p.split("=") for p in a.map.split(","))
    for line in sync(a.xlsx, read_results(path), day, smap):
        print(line)
