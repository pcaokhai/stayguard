# 15 — Production Design Specification

Version 1.0 · 2026-10-02 · Owner: Khai
Source of truth for screens: the design canvas, page **Production** (link in docs/README.md; exported PNG and markup per board in `docs/assets/design/`, with a board → route → task table in `docs/assets/design/INDEX.md`). UI libraries and motion: docs/16. Delivery plan: docs/14. This document turns those screens into rules, routes, API operations (contracts/openapi.yaml 1.1.0) and stories (docs/06, epics E7 to E13). Where this document and older docs disagree, this one wins for production scope; docs/14 (SHIP MODE) governs how and in what order the work is done.

## 1. How to read the canvas

| Label | Meaning |
| --- | --- |
| `Pnn · …` | Phone layout (390 px) of a production screen |
| `PC · …` | Desktop layout (1280 px) of the same screen or a desktop-only screen |
| `TAB · …` | Tablet sample (834 px portrait, 1194 px landscape) |
| `DOC · …` | Design document, not an app screen (responsive rules, SePay handover checklist) |
| `EN · …` | English version; every screen exists in Vietnamese and English |
| Demo page | The original 27 demo screens; several were updated and are part of production (room map, check-in, details, check-out, QR, paid, housekeeping list, owner overview phone, permissions, shift close, shift review) |

**Phone, tablet and desktop boards are one responsive page at three widths, not separate apps** (board "DOC · Quy tắc responsive"). One route and one component tree per page; layout changes at Tailwind `md:` (640 px) and `lg:` (1024 px). Every page is checked at 390, 834 and 1280 px in both languages.

| Pattern | Phone < 640 | Tablet 640–1023 | Desktop ≥ 1024 |
| --- | --- | --- | --- |
| Frame | Bottom tab bar per role on top-level pages (owner: Overview, Rooms, Payments, Alerts, More; front desk: Rooms, History, Shift, Schedule, Account; housekeeping: Clean, Schedule, Account); sub-pages and flows use a top bar with back and no tab bar | 88 px icon rail | 248 px grouped sidebar (owner) or top bar (front desk) |
| Owner "More" | Sheet with the same five groups as the desktop sidebar (board MenuChu) | Rail scrolls | All visible |
| Room map | 3-column grid, tap opens a page | 5 columns + 300 px detail panel | 6 columns + 320–440 px detail panel |
| Tables | Cards; secondary columns become a second line | Compact table | Full table; wide tables scroll sideways with a sticky first column |
| Add and edit forms | Full page | Right drawer | Right drawer or dialog |
| Confirmations, date pickers | Bottom sheet | Centred dialog | Centred dialog |
| Building selector | Horizontally scrolling chips | Same | Same |

## 2. Roles, positions and app access

Position (job) and app access (permissions) are separate fields on a staff member.

| App access | Who | Can | Cannot |
| --- | --- | --- | --- |
| OWNER | Owner | Everything | — |
| MANAGER | Optional manager | Everything the owner does on rooms, stays, stock, roster, maintenance, alerts | Bank accounts, removing staff, payroll and expense amounts, linking unmatched transfers |
| RECEPTIONIST | Front desk | Room map, check-in, extras, check-out, payments, cleaning, damage reports, own shift, own schedule and leave, stay history today and yesterday | Owner pages |
| HOUSEKEEPING | Housekeeping | Housekeeping list, cleaning, damage and used-room reports, own schedule and leave | Money screens |
| NONE | For example security | No sign-in; appears in roster and payroll only | Everything in the app |

Positions: FRONT_DESK, HOUSEKEEPING, SECURITY, MANAGER, MAINTENANCE, OTHER. Building access (NONE, VIEW, EDIT) applies only when app access is not NONE (ADR-008).

## 3. Product rules decided in design

Money and stays
1. A transfer is paid only from a bank event (simulator in demo). Unmatched transfers (wrong or missing bill code) can be linked to an unpaid invoice **by the owner only**, only for bank-reported money, irreversibly, with an audit entry.
2. QR codes expire after 30 minutes by default (property setting); money paid to an old code is still recorded when the bank reports it.
3. Check-in time may be corrected once per change, at most 60 minutes later than recorded, never in the future, with a reason code and note; this raises a STAY_TIME_EDITED alert. Check-out time is never editable.
4. Moving a guest keeps check-in time and extras; the whole stay is priced with the new room type; the old room becomes TO_CLEAN.
5. Stay history is chosen by day (previous and next day buttons, a date picker, quick Today and Yesterday). Receptionists can pick any day within the last N days (property setting, default 7); owners and managers can pick any range and open the stay timeline. Every list shows two ID columns: ID number on file (yes or no) and ID photos (front, back), never the data itself.

Rooms, cleaning, maintenance
6. Front desk, housekeeping, manager and owner can mark a TO_CLEAN room clean if they have EDIT on its building.
7. A damage report creates a maintenance ticket and an owner alert. "Lock the room" sets MAINTENANCE immediately (not allowed while a guest is in the room).
8. The owner enters parts and labour costs and an expected date; setting a ticket DONE posts a MAINTENANCE expense for that month and reopens the room.
9. Rooms can be added singly or as a range; floors and buildings can generate rooms. A room with a guest cannot change type or be retired. New buildings start with no staff access.

Stock
10. Adding an item records price, unit cost and opening quantity. Stock changes only through restock, sales and stocktake, each with a history row. Editing an item never edits stock.
11. Removing an item that has ever been sold sets it to "stop selling" so old bills keep its name and price.

People
12. Sign-in with guesthouse code, user name and a 6-digit PIN. Five wrong PINs lock for 15 minutes and alert the owner. One-time PINs (24 h) are shown once; the person must set their own PIN at first sign-in.
13. Removing staff deactivates the account (signed out at once); history and payroll are kept; an open shift must be closed first. Removing staff and bank-account changes require the owner's PIN again.
14. Roster: shifts MORNING 06–14, AFTERNOON 14–22, NIGHT 22–06. Staff request leave (paid, sick, unpaid) with optional cover; pending requests can be cancelled by the requester; approved leave needs the owner's approval to cancel. Uncovered shifts are flagged.
15. Payroll per month from each contract (monthly, per shift or hourly rate, fixed allowance, standard shifts, annual leave days) and the roster. Paid leave and sick leave are paid like worked shifts; unpaid leave is not paid (so it reduces a monthly salary in proportion to the standard shifts). Only paid leave uses up annual leave days; sick leave does not. Bonus and deduction are entered by the owner; the product never deducts cash shortages automatically (check current labour rules). Marking paid posts STAFF_PAY expense.

Finance
16. Expense categories: STAFF_PAY, RENT, ELECTRICITY, WATER, LAUNDRY, MAINTENANCE, SUPPLIES, COST_OF_GOODS, TAX_FEES, INTERNET_TV, PAYMENT_FEES, OTHER. Sources: MANUAL, RECURRING (auto-added each month), PAYROLL, MAINTENANCE, STOCK (cost of goods sold). Automatic lines cannot be edited by hand.
17. Cash paid from the drawer during a shift is a front-desk payout (shift reconciliation), not an owner expense, so nothing is counted twice.
18. The income and cost report takes a month range (quick picks: this month, last month, this quarter, 6 months, this year) and shows revenue, expenses, profit, margin, occupancy, monthly bars, expenses by category, revenue by rental type, building and payment method.

Bank and SePay
19. The owner can add and remove receiving accounts. QR always uses the default account; an account can become default only after the installer connects SePay for it. The default account cannot be removed.
Guest ID (optional)
21. At check-in the front desk may record the guest's national ID (CCCD) number and photos of the front and back. All three are optional; storing any of them requires ticking "guest agrees". Photos can be added or retaken while the stay is active or up to 24 hours after check-out.
22. After saving, the front desk and housekeeping can never read the number or photos again. Their screens only show indicators: "ID number on file", "front photo on file", "back photo on file" (or missing).
23. Only OWNER and MANAGER can see them. The number is masked by default (first 3 and last 3 digits) with a Show button; photos open in a viewer with Download and Delete. Every reveal, view, download and delete writes a GUEST_ID audit entry.
24. Photos are stripped of metadata, re-encoded, encrypted at rest and streamed only through the API with `Cache-Control: no-store` (no public or pre-signed links). Numbers are encrypted like other personal data.
25. Guest ID data is deleted automatically N days after check-out (property setting, default 30). Check the current Vietnamese personal-data protection rules and the stay-declaration requirements before go-live; the consent text and retention default may need to change.

20. SePay webhook path is per tenant (`/v1/webhooks/bank/{hookId}`). Secrets are written only through the installer CLI over SSH (docs/runbooks/sepay-handover.md). No screen or endpoint shows or edits a secret.

## 4. Screen inventory, routes and operations

Routes follow ADR-011: locale prefix, static export, ids in the query string. The same route renders phone, tablet and desktop layouts.

| Area | Canvas boards | Route | Main operations |
| --- | --- | --- | --- |
| Sign-in | P1, P2, P3, PC Đăng nhập | `/vi/sign-in`, `/vi/set-pin` | signIn, changeMyPin |
| Account | P23 Tài khoản | `/vi/account` | getMe, setMyLocale, signOut |
| Room map (front desk) | Demo 1, Sơ đồ máy tính, TAB Sơ đồ phòng | `/vi/rooms?b=` | listBuildings, listRooms |
| Room map (owner) | P41, PC Sơ đồ phòng | `/vi/owner/rooms?b=` | listBuildings, listRooms (no shift actions) |
| Check-in, details, extras, check-out | Demo 2–4, PC Nhận phòng, Thêm dịch vụ, Trả phòng | `/vi/checkin?room=`, `/vi/stay?id=`, `/vi/checkout?stay=` | createStay, getStay, addStayExtras, checkoutStay |
| Edit check-in, move room | P4, P5 | `/vi/stay/edit-time?id=`, `/vi/stay/move?id=` | editCheckInTime, moveStay |
| Payment | Demo 5–6, P6, P7, P8, PC Thanh toán | `/vi/pay?payment=`, `/vi/receipt?invoice=` | createPayment, getPayment, getReceipt |
| Guest ID capture and viewing | Demo 2 Nhận phòng, PC Nhận phòng (capture); Demo 3 Chi tiết, Sơ đồ máy tính (indicators); P42, PC Chi tiết lượt ở, P59, PC Xem ảnh CCCD (owner) | part of `/vi/checkin`, `/vi/owner/stay?id=` | createStay, setGuestIdNumber, uploadGuestIdPhoto, getGuestIdRecord, revealGuestIdNumber, getGuestIdPhoto, deleteGuestIdPhoto, deleteGuestIdNumber |
| Stay history | P9, PC Lịch sử (lễ tân) | `/vi/stays` | listStays |
| Cleaning | P37, P48, P47, PC Dọn phòng | `/vi/clean?room=`, `/vi/housekeeping` | listHousekeepingTasks, completeHousekeepingTask |
| Damage and used-room reports | P49, P27 | `/vi/report-damage?room=`, `/vi/report-used?room=` | reportDamage, reportRoomUsage |
| Front-desk shift | Demo Kết ca, P24, PC Chi tiền, PC Kết ca | `/vi/shift`, `/vi/shift/payout` | getCurrentShift, recordCashPayout, closeShift |
| My schedule and leave | P51, P52, P57, P58, PC Lịch và nghỉ (lễ tân) | `/vi/me/schedule`, `/vi/me/leave` | getMyRoster, listMyLeaveRequests, createLeaveRequest, cancelMyLeave |
| Owner overview | P60 Tổng quan (phone), TAB Tổng quan, PC Tổng quan | `/vi/owner` | getOwnerOverview |
| Alerts | P10, PC Cảnh báo, P32 empty state | `/vi/owner/alerts` | listAlerts, markAlertRead |
| Transactions, link money | P12, P43, PC Giao dịch, PC Gán tiền | `/vi/owner/transactions` | listTransactions, linkTransferToInvoice |
| Stay history and timeline (owner) | P42, PC Lịch sử lượt ở, PC Chi tiết lượt ở | `/vi/owner/stays`, `/vi/owner/stay?id=` | listStays, getStayTimeline |
| Shift reconciliation | Demo 12, P25, PC Đối soát ca | `/vi/owner/shifts`, `/vi/owner/shift?id=` | listClosedShifts, getShiftReview |
| Activity log | P13, P36, PC Nhật ký | `/vi/owner/activity` | listAuditLogs |
| Income and costs | P11, PC Báo cáo thu chi | `/vi/owner/reports` | getIncomeCostReport |
| Expenses | P55, P56, PC Chi phí, PC Thêm chi phí | `/vi/owner/expenses?month=` | getExpenseMonth, createExpense, updateExpense, deleteExpense |
| Payroll | PC Bảng lương | `/vi/owner/payroll?month=` | getPayroll, updatePayrollLine, markPayrollPaid |
| Maintenance | P53, P54, PC Bảo trì, PC Phiếu bảo trì | `/vi/owner/maintenance` | listTickets, getTicket, updateTicket |
| Extras and stock | P18, P44, P45, P46, P40, P28, PC Dịch vụ và kho, Quản lý mặt hàng, Sửa, Nhập thêm, Xóa, Kiểm kho | `/vi/owner/items`, `/vi/owner/item?code=` | listServices, createService, updateService, restockService, listStockMovements, removeService, createStocktake |
| Staff | P19, P20, P21, P39, PC Nhân viên, Thêm, PIN, Xóa | `/vi/owner/staff` | listStaff, createStaff, updateStaff, resetStaffPin, lockStaff, unlockStaff, removeStaff |
| Roster and leave (owner) | P50, PC Lịch ca và nghỉ | `/vi/owner/roster?week=` | getRoster, putRoster, copyRosterWeek, listLeaveRequests, approveLeave, declineLeave |
| Building access | Demo 10, PC Phân quyền | `/vi/owner/access` | listStaffPermissions, setBuildingPermission |
| Property and bank | P15, P38, PC Nhà nghỉ và ngân hàng, Thêm tài khoản | `/vi/owner/property` | getProperty, updateProperty, listBankAccounts, createBankAccount, makeDefaultBankAccount, removeBankAccount, getSepayStatus |
| Buildings and rooms | P16, P33, P34, P35, P26, PC Tòa và phòng | `/vi/owner/buildings?b=` | createBuilding, updateBuilding, createFloor, createRooms, updateRoom |
| Rates | P17, PC Bảng giá | `/vi/owner/rates` | listRatePlans, updateRatePlan, previewPrice |
| Common states | P29 to P32 | shared components | — |

Exact P numbers are on the canvas; numbering is stable once assigned.

## 5. Implementation order (production v1.1)

The executable plan with tasks, lanes and gates is docs/14. The epics below are the product view of the same work.

Detailed stories are in docs/06 (E7 to E13). Suggested order, each a vertical slice (API and screen together):

1. E7 sign-in and accounts, then E13 responsive shell (sidebar, icon rail, top bar) so every later page lands in the right frame.
2. E8 front-desk extras (edit time, move room, history, receipt, cleaning by front desk, damage reports, guest ID capture SG-805).
3. E9 owner monitoring (overview v2, alerts, transactions and linking, timeline, shift list, activity log).
4. E10 setup (property and bank accounts, buildings and rooms, rates, items and stock).
5. E11 people (staff with positions and contracts, roster and leave, self-service leave, payroll).
6. E12 finance and maintenance (tickets, expenses, income and cost report).

## 6. Open questions for the owner

- Q-01 Manager role: confirm the exclusions in §2.
- Q-02 (answered) Leave: paid leave counts as worked and uses annual leave days; sick leave is paid like worked shifts and does not use annual leave days; unpaid leave is unpaid and reduces monthly pay in proportion. Rule 15.
- Q-03 Recurring expense amounts that vary (electricity) are always manual; confirm.
- Q-04 Whether managers may enter maintenance costs.
- Q-05 Receipt printer model (58 or 80 mm) for print CSS.
- Q-06 Guest ID: final consent wording and retention period after checking current personal-data rules.
