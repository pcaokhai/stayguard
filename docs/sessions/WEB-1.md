# Session WEB-1

Paste everything below the line into a fresh Claude Code session started in the `stayguard-web1` clone.

---

You are session **WEB-1** of the StayGuard production sprint (SHIP MODE). You own `web/` shared foundation (`src/components/ui`, `src/components/shell`, `src/components/motion`, `src/lib`), the front desk, housekeeping and staff self-service screens.

Read CLAUDE.md, web/CLAUDE.md, docs/16-ui-kit-and-motion.md §1–6 and docs/14-production-sprint.md §1–6 now. Then do these tasks from docs/14 §5, in this order, one task at a time, using `/clear`-style focus: forget the previous task's details before starting the next.

Start W0 immediately (it does not need the API). Push W0 as soon as it is green: WEB-2 is waiting for it. After W0, WEB-2 may ask for changes to shared components in its reports; Khai forwards them to you, and you make them between tasks. Build on mocks; switch a screen to the real API once its API task is on main.

1. **W0** — UI kit, shells and motion foundation (WEB-1)
   Boards (Vietnamese and English): `MatMang` + `MatMangEN`, `LoiMayChu` + `LoiMayChuEN`, `KhongCoQuyen` + `KhongCoQuyenEN`, `DanhSachTrong` + `DanhSachTrongEN`, `QuyTacResponsive` + `QuyTacResponsiveEN`, `ThuVienUI` + `ThuVienUIEN`, `DieuHuongMobile` + `DieuHuongMobileEN`, `MenuChu` + `MenuChuEN`
2. **L-W1** — Sign-in and account (WEB-1)
   Boards (Vietnamese and English): `DangNhap` + `DangNhapEN`, `DangNhapPC` + `DangNhapPCEN`, `DoiPin` + `DoiPinEN`, `KhoaTaiKhoan` + `KhoaTaiKhoanEN`, `TaiKhoan` + `TaiKhoanEN`
3. **L-W2** — Room maps (WEB-1)
   Boards (Vietnamese and English): `Main` + `MainEN`, `SoDoMayTinh` + `SoDoMayTinhEN`, `SoDoPhongTab` + `SoDoPhongTabEN`, `SoDoPhongChu` + `SoDoPhongChuEN`, `SoDoPhongChuPC` + `SoDoPhongChuPCEN`
4. **L-W3** — Stay flows (WEB-1)
   Boards (Vietnamese and English): `NhanPhong` + `NhanPhongEN`, `NhanPhongPC` + `NhanPhongPCEN`, `ChiTiet` + `ChiTietEN`, `ThemDichVu` + `ThemDichVuEN`, `ThemDichVuPC` + `ThemDichVuPCEN`, `SuaGio` + `SuaGioEN`, `ChuyenPhong` + `ChuyenPhongEN`, `TraPhong` + `TraPhongEN`, `TraPhongPC` + `TraPhongPCEN`, `ThanhToanQR` + `ThanhToanQREN`, `ThanhToanPC` + `ThanhToanPCEN`, `ChuyenKhoanLech` + `ChuyenKhoanLechEN`, `QRHetHan` + `QRHetHanEN`, `DaThanhToan` + `DaThanhToanEN`, `BienLai` + `BienLaiEN`
5. **L-W4** — Shift and history (WEB-1)
   Boards (Vietnamese and English): `ChiTrongCa` + `ChiTrongCaEN`, `ChiTrongCaPC` + `ChiTrongCaPCEN`, `GiaoCa` + `GiaoCaEN`, `GiaoCaPC` + `GiaoCaPCEN`, `LichSuLuotO` + `LichSuLuotOEN`, `LichSuLeTanPC` + `LichSuLeTanPCEN`
6. **L-W5** — Cleaning and reports (WEB-1)
   Boards (Vietnamese and English): `PhongCanDon` + `PhongCanDonEN`, `PhongCanDonPC` + `PhongCanDonPCEN`, `BuongPhongP` + `BuongPhongPEN`, `PhongCanDonBP` + `PhongCanDonBPEN`, `BaoHuHong` + `BaoHuHongEN`, `BaoPhongDung` + `BaoPhongDungEN`
7. **F-W1** — Guest ID UI (WEB-1)
   Boards (Vietnamese and English): `XemGiayTo` + `XemGiayToEN`, `XemGiayToPC` + `XemGiayToPCEN`, `NhanPhong` + `NhanPhongEN`, `NhanPhongPC` + `NhanPhongPCEN`, `ChiTiet` + `ChiTietEN`, `SoDoMayTinh` + `SoDoMayTinhEN`, `ChiTietLuotO` + `ChiTietLuotOEN`, `ChiTietLuotOPC` + `ChiTietLuotOPCEN`, `SoDoPhongChuPC` + `SoDoPhongChuPCEN`, `LichSuLuotO` + `LichSuLuotOEN`, `LichSuLeTanPC` + `LichSuLeTanPCEN`, `LichSuLuotOPC` + `LichSuLuotOPCEN`, `CaiDatNhaNghi` + `CaiDatNhaNghiEN`, `CaiDatNhaNghiPC` + `CaiDatNhaNghiPCEN`
8. **F-W2** — My schedule and leave (WEB-1)
   Boards (Vietnamese and English): `LichCuaToi` + `LichCuaToiEN`, `XinNghi` + `XinNghiEN`, `NghiCuaToi` + `NghiCuaToiEN`, `HuyNghi` + `HuyNghiEN`, `LichNghiLeTanPC` + `LichNghiLeTanPCEN`

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
