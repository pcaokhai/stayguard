# Session WEB-2

Paste everything below the line into a fresh Claude Code session started in the `stayguard-web2` clone.

---

You are session **WEB-2** of the StayGuard production sprint (SHIP MODE). You own `web/` owner and settings pages: overview, alerts, activity log, transactions, stay history, shifts, staff and access, property and bank, buildings and rooms, rates, extras and stock, maintenance, expenses, payroll, reports and roster.

Read CLAUDE.md, web/CLAUDE.md, docs/16-ui-kit-and-motion.md §1–6 and docs/14-production-sprint.md §1–6 now. Then do these tasks from docs/14 §5, in this order, one task at a time, using `/clear`-style focus: forget the previous task's details before starting the next.

Before L-W6: `git pull --rebase` until the W0 commit from WEB-1 is on main. Do not edit `src/components/ui`, `src/components/shell`, `src/components/motion` or `src/lib`; if you need a change there, build it locally in your feature folder and list the request in your report. Build on mocks; switch a page to the real API once its API task is on main.

1. **L-W6** — Owner shell, overview, alerts, activity log (WEB-2)
   Boards (Vietnamese and English): `TongQuan` + `TongQuanEN`, `TongQuanTab` + `TongQuanTabEN`, `TongQuanPC` + `TongQuanPCEN`, `CanhBao` + `CanhBaoEN`, `CanhBaoPC` + `CanhBaoPCEN`, `NhatKy` + `NhatKyEN`, `NhatKyChonNgay` + `NhatKyChonNgayEN`, `NhatKyPC` + `NhatKyPCEN`
2. **L-W7** — Money and shifts for the owner (WEB-2)
   Boards (Vietnamese and English): `GiaoDichPC` + `GiaoDichPCEN`, `LichSuGiaoDich` + `LichSuGiaoDichEN`, `GanPhieu` + `GanPhieuEN`, `GanPhieuPC` + `GanPhieuPCEN`, `LichSuLuotOPC` + `LichSuLuotOPCEN`, `ChiTietLuotO` + `ChiTietLuotOEN`, `ChiTietLuotOPC` + `ChiTietLuotOPCEN`, `ID` + `IDEN`, `DanhSachCa` + `DanhSachCaEN`, `DoiSoatCa` + `DoiSoatCaEN`, `DoiSoatCaPC` + `DoiSoatCaPCEN`
3. **L-W8** — People (WEB-2)
   Boards (Vietnamese and English): `NhanVien` + `NhanVienEN`, `NhanVienPC` + `NhanVienPCEN`, `ThemNhanVien` + `ThemNhanVienEN`, `ThemNhanVienPC` + `ThemNhanVienPCEN`, `PinMotLan` + `PinMotLanEN`, `PinMotLanPC` + `PinMotLanPCEN`, `XoaNhanVien` + `XoaNhanVienEN`, `XoaNhanVienPC` + `XoaNhanVienPCEN`, `PhanQuyen` + `PhanQuyenEN`, `PhanQuyenPC` + `PhanQuyenPCEN`
4. **L-W9** — Settings (WEB-2)
   Boards (Vietnamese and English): `CaiDat` + `CaiDatEN`, `CaiDatNhaNghi` + `CaiDatNhaNghiEN`, `CaiDatNhaNghiPC` + `CaiDatNhaNghiPCEN`, `ThemNganHang` + `ThemNganHangEN`, `ThemNganHangPC` + `ThemNganHangPCEN`, `ToaPhong` + `ToaPhongEN`, `ToaPhongPC` + `ToaPhongPCEN`, `ThemPhong` + `ThemPhongEN`, `ThemTang` + `ThemTangEN`, `ThemToa` + `ThemToaEN`, `SuaPhong` + `SuaPhongEN`, `BangGia` + `BangGiaEN`, `BangGiaPC` + `BangGiaPCEN`, `DichVuKho` + `DichVuKhoEN`, `DichVuKhoPC` + `DichVuKhoPCEN`, `ThemMatHang` + `ThemMatHangEN`, `ThemMatHangPC` + `ThemMatHangPCEN`, `SuaMatHang` + `SuaMatHangEN`, `SuaMatHangPC` + `SuaMatHangPCEN`
5. **F-W3** — Stock pages (WEB-2)
   Boards (Vietnamese and English): `ChiTietMatHang` + `ChiTietMatHangEN`, `ChiTietMatHangPC` + `ChiTietMatHangPCEN`, `NhapThemHang` + `NhapThemHangEN`, `NhapThemHangPC` + `NhapThemHangPCEN`, `XoaMatHangPC` + `XoaMatHangPCEN`, `KiemKho` + `KiemKhoEN`, `KiemKhoPC` + `KiemKhoPCEN`
6. **F-W4** — Maintenance and finance pages (WEB-2)
   Boards (Vietnamese and English): `BaoTri` + `BaoTriEN`, `BaoTriPC` + `BaoTriPCEN`, `BaoTriChiTiet` + `BaoTriChiTietEN`, `BaoTriChiTietPC` + `BaoTriChiTietPCEN`, `ChiPhi` + `ChiPhiEN`, `ChiPhiPC` + `ChiPhiPCEN`, `ThemChiPhi` + `ThemChiPhiEN`, `ThemChiPhiPC` + `ThemChiPhiPCEN`, `BangLuongPC` + `BangLuongPCEN`, `BaoCao` + `BaoCaoEN`, `BaoCaoPC` + `BaoCaoPCEN`
7. **F-W5** — Roster for the owner, final language sweep (WEB-2)
   Boards (Vietnamese and English): `LichCa` + `LichCaEN`, `LichCaPC` + `LichCaPCEN`

Rules for this session:
- You are one of four parallel sessions; this clone is only yours. Before each task `git pull --rebase`; after it, one commit `<type>(<api|web>): <summary> (<task id>)` that also ticks the task in docs/14, then `git pull --rebase && git push`. If the rebase conflicts in a file another lane owns, keep their version and redo your part.
- Start every task by re-reading its section in docs/14 §5. Read docs/15 or docs/16 sections only when the task needs them. Never read docs/archive.
- Keep the strict rules in CLAUDE.md §4. Write the tests listed in docs/14 §6 for your task first.
- Before ticking a task, run its verification and paste the last lines of real output in your report.
- If you are blocked for more than 15 minutes (missing decision, external account, failing dependency from another lane), write the blocker in your report, leave the box unticked, and move to the next task that does not depend on it.
- After each task, report in at most 8 lines: what changed, what you ran, what Khai must check by hand, and (UI tasks) each board as match or differences in vi and en.
- Continue with the next task without waiting for me unless the task says Khai must decide or review first.

UI rules (every UI task):
- Components from shadcn/ui and the libraries in docs/16 §1; motion from docs/16 §4–5; no hand-built primitives.
- Every string through t(); add keys to messages/vi.json and messages/en.json together. Copy English text from docs/assets/design/source/<Board>EN.dc.html.
- One route serves phone, tablet and desktop (docs/15 §1). Phone boards are 390 px, tablet 834/1194, desktop 1280.
- Check each board with agent-browser (docs/16 §6): `scripts/ui-shots.sh /vi/<route> <Board> <WxH>` and `scripts/ui-shots.sh /en/<route> <Board>EN <WxH>` at 390x844, 834x1194 and 1280x900; routes are in docs/assets/design/INDEX.md. Fix wrong data, text, missing states and broken layout; small spacing or font differences are fine.
- `npm run build`, `npm run lint`, `npx playwright test e2e/layout.spec.ts` green before each commit. Mock mode must keep working: add realistic data for new screens in src/mocks/setup/demo-handlers.ts.

When the list is done, run the full verification for your lane once more (`npm run build`, `npm run lint`, `npx playwright test`, and an agent-browser sweep of every route you built in vi and en at 390 and 1280 px) and report the result.
