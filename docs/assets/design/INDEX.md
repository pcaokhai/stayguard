# Design index (production)

Every production board in both languages: Vietnamese `screens/<Board>.png` and English `screens/<Board>EN.png` (rendered at 1x without the web font), with markup in `source/<Board>.dc.html` and `source/<Board>EN.dc.html` (open with `support.js` beside it). The Vietnamese board is checked on `/vi/…`, the English board on the same route under `/en/…`. English strings for `messages/en.json` are copied from the EN markup.

Width 390 = phone layout, 834/1194 = tablet, 1280 = desktop. They are the same route at different widths (docs/15 §1). The task column points to docs/14.

| Board (vi; add `EN` for English) | Title on canvas | Width | Route (`/vi` or `/en`) | Task |
| --- | --- | --- | --- | --- |
| `BangGia` | P17 · Bảng giá | 390 | /vi/owner/rates | L-W9 |
| `BangGiaPC` | PC · Bảng giá | 1280 | /vi/owner/rates (≥1024) | L-W9 |
| `BangLuongPC` | PC · Bảng lương | 1280 | /vi/owner/payroll?month= | F-W4 |
| `BaoCao` | P11 · Báo cáo thu chi | 390 | /vi/owner/reports | F-W4 |
| `BaoCaoPC` | PC · Báo cáo thu chi | 1280 | /vi/owner/reports (≥1024) | F-W4 |
| `BaoHuHong` | P49 · Báo hư hỏng | 390 | /vi/report-damage?room= | L-W5 |
| `BaoPhongDung` | P27 · Báo phòng có dấu hiệu dùng | 390 | /vi/report-used?room= | L-W5 |
| `BaoTri` | P53 · Bảo trì, sửa chữa | 390 | /vi/owner/maintenance | F-W4 |
| `BaoTriChiTiet` | P54 · Phiếu bảo trì | 390 | /vi/owner/ticket?id= | F-W4 |
| `BaoTriChiTietPC` | PC · Phiếu bảo trì | 1280 | /vi/owner/maintenance (drawer) | F-W4 |
| `BaoTriPC` | PC · Bảo trì, sửa chữa | 1280 | /vi/owner/maintenance (≥1024) | F-W4 |
| `BienLai` | P8 · Biên lai | 390 | /vi/receipt?invoice= | L-W3 |
| `BuongPhong` | 8 · Buồng phòng | 390 | /vi/housekeeping (old) | L-W5 |
| `BuongPhongP` | P47 · Buồng phòng | 390 | /vi/housekeeping | L-W5 |
| `CaiDat` | P14 · Cài đặt | 390 | /vi/owner/settings | L-W9 |
| `CaiDatNhaNghi` | P15 · Nhà nghỉ và ngân hàng | 390 | /vi/owner/property | L-W9 |
| `CaiDatNhaNghiPC` | PC · Nhà nghỉ và ngân hàng | 1280 | /vi/owner/property (≥1024) | L-W9 |
| `CanhBao` | P10 · Cảnh báo | 390 | /vi/owner/alerts | L-W6 |
| `CanhBaoPC` | PC · Cảnh báo | 1280 | /vi/owner/alerts (≥1024) | L-W6 |
| `ChiPhi` | P55 · Chi phí | 390 | /vi/owner/expenses?month= | F-W4 |
| `ChiPhiPC` | PC · Chi phí | 1280 | /vi/owner/expenses (≥1024) | F-W4 |
| `ChiTiet` | 3 · Phòng có khách | 390 | /vi/stay?id= | L-W3 |
| `ChiTietLuotO` | P42 · Chi tiết lượt ở | 390 | /vi/owner/stay?id= | L-W7 |
| `ChiTietLuotOPC` | PC · Chi tiết lượt ở | 1280 | /vi/owner/stay?id= (≥1024) | L-W7 |
| `ChiTietMatHang` | P44 · Quản lý mặt hàng | 390 | /vi/owner/item?code= | F-W3 |
| `ChiTietMatHangPC` | PC · Quản lý mặt hàng | 1280 | /vi/owner/item?code= (≥1024) | F-W3 |
| `ChiTrongCa` | P24 · Ghi khoản chi | 390 | /vi/shift/payout | L-W4 |
| `ChiTrongCaPC` | PC · Chi tiền trên máy tính | 1280 | /vi/shift/payout (≥1024) | L-W4 |
| `ChonVaiTro` | 0 · Chọn vai trò (màn mở đầu) | 390 | /vi (demo only) | (demo only) |
| `ChuyenKhoanLech` | P6 · Chuyển khoản lệch tiền | 390 | /vi/pay?payment= (MISMATCH) | L-W3 |
| `ChuyenPhong` | P5 · Chuyển phòng | 390 | /vi/stay/move?id= | L-W3 |
| `DaThanhToan` | 6 · Đã thanh toán | 390 | /vi/paid?payment= | L-W3 |
| `DangNhap` | P1 · Đăng nhập | 390 | /vi/sign-in | L-W1 |
| `DangNhapPC` | PC · Đăng nhập | 1280 | /vi/sign-in (≥1024) | L-W1 |
| `DanhSachCa` | P25 · Các ca đã đóng | 390 | /vi/owner/shifts | L-W7 |
| `DanhSachTrong` | P32 · Danh sách trống | 390 | shared state | W0 |
| `DichVuKho` | P18 · Dịch vụ và kho | 390 | /vi/owner/items | L-W9 |
| `DichVuKhoPC` | PC · Dịch vụ và kho | 1280 | /vi/owner/items (≥1024) | L-W9 |
| `DoiPin` | P2 · Đặt PIN mới | 390 | /vi/set-pin | L-W1 |
| `DoiSoatCa` | 12 · Đối soát ca (chủ) | 390 | /vi/owner/shift?id= | L-W7 |
| `DoiSoatCaPC` | PC · Đối soát ca | 1280 | /vi/owner/shifts (≥1024) | L-W7 |
| `GanPhieu` | P43 · Gán tiền vào phiếu | 390 | /vi/owner/transactions (sheet) | L-W7 |
| `GanPhieuPC` | PC · Gán tiền vào phiếu | 1280 | /vi/owner/transactions (dialog) | L-W7 |
| `GiaoCa` | 11 · Kết ca (lễ tân) | 390 | /vi/shift | L-W4 |
| `GiaoCaPC` | PC · Kết ca trên máy tính | 1280 | /vi/shift (≥1024) | L-W4 |
| `GiaoDichPC` | PC · Giao dịch | 1280 | /vi/owner/transactions | L-W7 |
| `HuyNghi` | P58 · Hủy yêu cầu nghỉ | 390 | /vi/me/leave (sheet) | F-W2 |
| `KetNoiSePay` | DOC · Bàn giao SePay · phần của chủ | 390 | doc (installer) | L-A3 |
| `KetNoiSePayPC` | DOC · Bàn giao SePay · phần trên máy chủ | 1280 | doc (installer) | L-A3 |
| `KhoaTaiKhoan` | P3 · Tài khoản tạm khóa | 390 | /vi/sign-in (locked) | L-W1 |
| `KhongCoQuyen` | P31 · Không có quyền | 390 | shared state | W0 |
| `KiemKho` | P28 · Kiểm kho | 390 | /vi/owner/stocktake | F-W3 |
| `KiemKhoPC` | PC · Kiểm kho | 1280 | /vi/owner/stocktake (≥1024) | F-W3 |
| `LichCa` | P50 · Lịch ca và nghỉ (chủ) | 390 | /vi/owner/roster | F-W5 |
| `LichCaPC` | PC · Lịch ca và nghỉ | 1280 | /vi/owner/roster (≥1024) | F-W5 |
| `LichCuaToi` | P51 · Lịch của tôi | 390 | /vi/me/schedule | F-W2 |
| `LichNghiLeTanPC` | PC · Lịch và nghỉ (lễ tân, máy tính) | 1280 | /vi/me/schedule (≥1024) | F-W2 |
| `LichSuGiaoDich` | P12 · Giao dịch | 390 | /vi/owner/transactions (phone) | L-W7 |
| `LichSuLeTanPC` | PC · Lịch sử lượt ở (lễ tân) | 1280 | /vi/stays (≥1024) | L-W4 |
| `LichSuLuotO` | P9 · Lịch sử lượt ở | 390 | /vi/stays | L-W4 |
| `LichSuLuotOPC` | PC · Lịch sử lượt ở | 1280 | /vi/owner/stays | L-W7 |
| `LoiMayChu` | P30 · Lỗi máy chủ | 390 | shared state | W0 |
| `Main` | 1 · Sơ đồ phòng | 390 | /vi/rooms | L-W2 |
| `MatMang` | P29 · Mất kết nối mạng | 390 | shared state | W0 |
| `NghiCuaToi` | P57 · Yêu cầu nghỉ của tôi | 390 | /vi/me/leave | F-W2 |
| `NhanPhong` | 2 · Nhận phòng | 390 | /vi/checkin?room= | L-W3 |
| `NhanPhongPC` | PC · Nhận phòng trên máy tính | 1280 | /vi/checkin?room= (≥1024) | L-W3 |
| `NhanVien` | P19 · Nhân viên | 390 | /vi/owner/staff | L-W8 |
| `NhanVienPC` | PC · Nhân viên | 1280 | /vi/owner/staff (≥1024) | L-W8 |
| `NhapThemHang` | P46 · Nhập thêm hàng | 390 | /vi/owner/item?code= (sheet) | F-W3 |
| `NhapThemHangPC` | PC · Nhập thêm hàng | 1280 | /vi/owner/item?code= (dialog) | F-W3 |
| `NhatKy` | P13 · Nhật ký thao tác | 390 | /vi/owner/activity | L-W6 |
| `NhatKyChonNgay` | P36 · Chọn ngày xem nhật ký | 390 | /vi/owner/activity (date sheet) | L-W6 |
| `NhatKyPC` | PC · Nhật ký thao tác | 1280 | /vi/owner/activity (≥1024) | L-W6 |
| `PhanQuyen` | 10 · Phân quyền nhân viên | 390 | /vi/owner/access | L-W8 |
| `PhanQuyenPC` | PC · Phân quyền theo tòa | 1280 | /vi/owner/access (≥1024) | L-W8 |
| `PhongCanDon` | P37 · Dọn phòng | 390 | /vi/clean?room= | L-W5 |
| `PhongCanDonBP` | P48 · Dọn phòng (buồng phòng) | 390 | /vi/clean?room= (housekeeping) | L-W5 |
| `PhongCanDonPC` | PC · Dọn phòng trên máy tính | 1280 | /vi/clean?room= (≥1024) | L-W5 |
| `PinMotLan` | P21 · PIN dùng một lần | 390 | /vi/owner/staff (one-time PIN) | L-W8 |
| `PinMotLanPC` | PC · PIN dùng một lần | 1280 | /vi/owner/staff (one-time PIN) | L-W8 |
| `QRHetHan` | P7 · Mã QR hết hạn | 390 | /vi/pay?payment= (EXPIRED) | L-W3 |
| `QuyTacResponsive` | DOC · Quy tắc responsive | 1280 | doc | W0 |
| `SoDoMayTinh` | 7 · Sơ đồ phòng trên máy tính lễ tân | 1280 | /vi/rooms (≥1024) | L-W2 |
| `SoDoPhongChu` | P41 · Sơ đồ phòng của chủ | 390 | /vi/owner/rooms | L-W2 |
| `SoDoPhongChuPC` | PC · Sơ đồ phòng của chủ | 1280 | /vi/owner/rooms (≥1024) | L-W2 |
| `SoDoPhongTab` | TAB · Sơ đồ phòng · tablet ngang | 1194 | /vi/rooms (tablet) | L-W2 |
| `SuaGio` | P4 · Sửa giờ vào | 390 | /vi/stay/edit-time?id= | L-W3 |
| `SuaMatHang` | P45 · Sửa mặt hàng | 390 | /vi/owner/item/edit?code= | L-W9 |
| `SuaMatHangPC` | PC · Sửa mặt hàng | 1280 | /vi/owner/item?code= (dialog) | L-W9 |
| `SuaPhong` | P26 · Sửa phòng | 390 | /vi/owner/room?id= | L-W9 |
| `TaiKhoan` | P23 · Tài khoản | 390 | /vi/account | L-W1 |
| `ThanhToanPC` | PC · Thanh toán QR trên máy tính | 1280 | /vi/pay?payment= (≥1024) | L-W3 |
| `ThanhToanQR` | 5 · Thanh toán QR | 390 | /vi/pay?payment= | L-W3 |
| `ThemChiPhi` | P56 · Thêm chi phí | 390 | /vi/owner/expenses/new | F-W4 |
| `ThemChiPhiPC` | PC · Thêm chi phí | 1280 | /vi/owner/expenses (drawer) | F-W4 |
| `ThemDichVu` | 3b · Thêm dịch vụ | 390 | /vi/stay?id= (sheet) | L-W3 |
| `ThemDichVuPC` | PC · Thêm dịch vụ trên máy tính | 1280 | /vi/stay?id= (≥1024) | L-W3 |
| `ThemMatHang` | P40 · Thêm mặt hàng | 390 | /vi/owner/items/new | L-W9 |
| `ThemMatHangPC` | PC · Thêm mặt hàng | 1280 | /vi/owner/items (drawer) | L-W9 |
| `ThemNganHang` | P38 · Thêm tài khoản nhận tiền | 390 | /vi/owner/property/bank-new | L-W9 |
| `ThemNganHangPC` | PC · Thêm tài khoản nhận tiền | 1280 | /vi/owner/property (drawer) | L-W9 |
| `ThemNhanVien` | P20 · Thêm nhân viên | 390 | /vi/owner/staff/new | L-W8 |
| `ThemNhanVienPC` | PC · Thêm nhân viên | 1280 | /vi/owner/staff (drawer) | L-W8 |
| `ThemPhong` | P33 · Thêm phòng | 390 | /vi/owner/rooms/new | L-W9 |
| `ThemTang` | P34 · Thêm tầng | 390 | /vi/owner/floors/new | L-W9 |
| `ThemToa` | P35 · Thêm tòa | 390 | /vi/owner/buildings/new | L-W9 |
| `ThuVienUI` | DOC · Thư viện UI và chuyển động | 1280 | doc | W0 |
| `ToaPhong` | P16 · Tòa và phòng | 390 | /vi/owner/buildings | L-W9 |
| `ToaPhongPC` | PC · Tòa và phòng | 1280 | /vi/owner/buildings (≥1024) | L-W9 |
| `TongQuan` | P60 · Tổng quan của chủ (with bottom tab bar) | 390 | /vi/owner | L-W6 |
| `MenuChu` | P61 · Menu của chủ (Thêm) | 390 | owner bottom tab More (sheet) | W0 |
| `DieuHuongMobile` | DOC · Điều hướng trên điện thoại | 1280 | doc: phone bottom tab bars per role | W0 |
| `TongQuanChu` | 9 · Tổng quan của chủ | 390 | retired (old demo layout; use TongQuan) | — |
| `TongQuanPC` | PC · Tổng quan | 1280 | /vi/owner (≥1024) | L-W6 |
| `TongQuanTab` | TAB · Tổng quan · tablet dọc | 834 | /vi/owner (tablet) | L-W6 |
| `TraPhong` | 4 · Trả phòng | 390 | /vi/checkout?stay= | L-W3 |
| `TraPhongPC` | PC · Trả phòng trên máy tính | 1280 | /vi/checkout?stay= (≥1024) | L-W3 |
| `XemGiayTo` | P59 · Xem ảnh CCCD | 390 | /vi/owner/stay?id= (viewer) | F-W1 |
| `XemGiayToPC` | PC · Xem ảnh CCCD | 1280 | /vi/owner/stay?id= (viewer) | F-W1 |
| `XinNghi` | P52 · Xin nghỉ | 390 | /vi/me/leave/new | F-W2 |
| `XoaMatHangPC` | PC · Xóa mặt hàng | 1280 | /vi/owner/items (dialog) | F-W3 |
| `XoaNhanVien` | P39 · Xóa nhân viên | 390 | /vi/owner/staff (sheet) | L-W8 |
| `XoaNhanVienPC` | PC · Xóa nhân viên | 1280 | /vi/owner/staff (dialog) | L-W8 |
