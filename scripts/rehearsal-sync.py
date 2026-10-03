#!/usr/bin/env python3
"""Fills the Status, Date and Notes columns of docs/rehearsal/checklist.xlsx from a rehearsal results CSV, for the rows whose ID matches.
It never touches a row marked manual (any cell reading "manual"), never overwrites a formula, and edits the sheet XML in place, so
styles, formulas, validation and everything else in the workbook stay as Khai saved them. No third-party packages.

usage: scripts/rehearsal-sync.py [results.csv] [--xlsx docs/rehearsal/checklist.xlsx] [--date YYYY-MM-DD] [--map pass=,fail=Lỗi,skip=]
       scripts/rehearsal-sync.py --selftest
Without a CSV it uses the newest docs/rehearsal/results-*.csv. The sheet needs a header row with ID, Status, Date and Notes cells."""
import argparse, csv, datetime, glob, os, re, shutil, sys, tempfile, zipfile
from xml.sax.saxutils import escape, unescape

CELL = re.compile(r'<c r="([A-Z]+)(\d+)"([^>]*?)(?:/>|>(.*?)</c>)', re.S)
ROW = re.compile(r'(<row r="(\d+)"[^>]*?)(?:/>|>(.*?)</row>)', re.S)
HEADERS = {"id": {"id", "ma", "mã"}, "status": {"status", "trang thai", "trạng thái"}, "date": {"date", "ngay thu", "ngày thử"},
           "notes": {"notes", "note", "ghi chu", "ghi chú", "ghi chú, bằng chứng"}}
AUTO = "QA tự động"  # prefix of the one line this script keeps in the notes cell
MANUAL = {"manual", "thu cong", "thủ công"}
MANUAL_NOTE = re.compile(r"\[(manual|thủ công)\]", re.I)


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
    if isinstance(text, (int, float)):
        return f'<c r="{ref}"{style}><v>{text}</v></c>'
    return f'<c r="{ref}"{style} t="inlineStr"><is><t xml:space="preserve">{escape(text)}</t></is></c>'


def set_cell(row_xml, letters, rownum, text, default_style=""):
    """Returns (new row xml, written?). A formula cell is left alone."""
    ref = f"{letters}{rownum}"
    for m in CELL.finditer(row_xml):
        if m.group(1) == letters:
            if m.group(4) and "<f" in m.group(4):
                return row_xml, False
            style = re.search(r'\bs="\d+"', m.group(3))
            new = inline_cell(ref, (" " + style.group(0)) if style else "", text)
            return row_xml[: m.start()] + new + row_xml[m.end():], True
    new = inline_cell(ref, default_style, text)
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

        styles = {}  # style of the first filled cell per column, for cells the sheet has not got yet
        for m in CELL.finditer(xml):
            st = re.search(r'\bs="\d+"', m.group(3))
            if st and m.group(4) and m.group(1) not in styles and int(m.group(2)) > header_row:
                styles[m.group(1)] = " " + st.group(0)
        shown = f"{date[8:10]}/{date[5:7]}/{date[:4]}" if len(date) == 10 else date
        serial = (datetime.date.fromisoformat(date) - datetime.date(1899, 12, 30)).days if len(date) == 10 else date

        def edit(rm):
            head, rownum, body = rm.group(1), int(rm.group(2)), rm.group(3)
            if rownum <= header_row or body is None:
                return rm.group(0)
            row = f"{head}>{body}</row>"
            cells = {m.group(1): cell_text(m.group(3), m.group(4), sst).strip() for m in CELL.finditer(row)}
            rid = cells.get(cols["id"], "")
            if rid not in results:
                return rm.group(0)
            note_now = cells.get(cols.get("notes", ""), "")
            if any(t.lower() in MANUAL for t in cells.values()) or MANUAL_NOTE.search(note_now):
                report.append(f"{name}!{rid}: manual row, left alone")
                return rm.group(0)
            r, wrote = results[rid], []
            status = status_map.get(r["status"], "")
            detail = (r["notes"] or "").strip()
            line = f"{AUTO} {shown}: {r['status']}" + (f" ({detail})" if detail else "") + f", chưa thử tay; bằng chứng {r['evidence']}"
            kept = [ln for ln in note_now.split("\n") if ln and not ln.startswith(AUTO)]
            values = {"notes": "\n".join(kept + [line])}
            if status:  # a pass never writes Đạt: only a person who saw it does
                values["status"] = status
                values["date"] = serial
            for key, letters in cols.items():
                if key in values:
                    row, ok = set_cell(row, letters, rownum, values[key], styles.get(letters, ""))
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
    row = lambda r, cells: f'<row r="{r}">' + "".join(cells) + "</row>"
    t = lambda ref, text: f'<c r="{ref}" t="inlineStr"><is><t>{text}</t></is></c>'
    sheet = (
        '<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>'
        + row(1, [t("A1", "Mã"), t("B1", "Test case"), t("C1", "Trạng thái"), t("D1", "Ngày thử"), t("E1", "Ghi chú, bằng chứng")])
        + row(2, [t("A2", "TT-01"), '<c r="C2" s="3" t="inlineStr"><is><t>Đạt</t></is></c>', '<c r="D2" s="7"><v>46298</v></c>'])
        + row(3, [t("A3", "TT-02"), t("C3", "Chưa làm"), t("E3", "ghi tay\nQA tự động 01/01/2026: fail (cũ)")])
        + row(4, [t("A4", "TT-03"), t("B4", "manual"), t("C4", "giữ")])
        + row(5, [t("A5", "TT-04"), '<c r="C5"><f>1+1</f><v>2</v></c>'])
        + row(6, [t("A6", "TT-05"), t("C6", "Chưa làm")])
        + "</sheetData></worksheet>"
    )
    with zipfile.ZipFile(x, "w") as z:
        z.writestr("[Content_Types].xml", '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>')
        z.writestr("xl/workbook.xml", '<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Checklist" sheetId="1" r:id="rId1"/></sheets></workbook>')
        z.writestr("xl/_rels/workbook.xml.rels", '<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>')
        z.writestr("xl/worksheets/sheet1.xml", sheet)
    res = {k: {"id": k, "status": st, "notes": n, "evidence": "ev/" + k} for k, st, n in
           [("TT-01", "pass", ""), ("TT-02", "fail", "boom"), ("TT-03", "fail", ""), ("TT-04", "fail", "x"), ("TT-05", "pass", "")]}
    sync(x, res, "2026-10-03", {"pass": "", "fail": "Lỗi", "skip": ""})
    out = zipfile.ZipFile(x).read("xl/worksheets/sheet1.xml").decode()
    r = lambda n: out.split(f'<row r="{n}">')[1].split("</row>")[0]
    assert "Đạt" in r(2) and "<v>46298</v>" in r(2) and "QA tự động 03/10/2026: pass" in r(2), r(2)  # a pass never touches Đạt or the date
    assert ">Lỗi<" in r(3) and "ghi tay" in r(3) and "(cũ)" not in r(3) and "boom" in r(3), r(3)  # tester's note kept, old auto line replaced
    assert f"<v>{(datetime.date(2026, 10, 3) - datetime.date(1899, 12, 30)).days}</v>" in r(3), r(3)
    assert "QA tự động" not in r(4) and ">giữ<" in r(4), r(4)  # the manual row
    assert "<f>1+1</f>" in r(5) and "Lỗi" not in r(5), r(5)  # formula kept
    assert ">Chưa làm<" in r(6) and "pass" in r(6), r(6)  # status untouched on a pass
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
    ap.add_argument("--map", default="pass=,fail=Lỗi,skip=")
    a = ap.parse_args()
    path = a.csv or max(glob.glob("docs/rehearsal/results-*.csv"), default=None)
    if not path or not os.path.exists(a.xlsx):
        sys.exit(f"need a results CSV and {a.xlsx} (Khai commits the checklist there first)")
    day = a.date or (re.search(r"results-(\d{4}-\d{2}-\d{2})", path) or [None, ""])[1]
    smap = dict(p.split("=") for p in a.map.split(","))
    for line in sync(a.xlsx, read_results(path), day, smap):
        print(line)
