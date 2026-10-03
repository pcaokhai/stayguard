# Rehearsal bugs (QA, portfolio demo)

Found by `make rehearse-test` (`docs/rehearsal/results-<date>.csv`, evidence under `evidence-<date>/<id>/`). Owner is the lane that fixes it.
Status: **open**, or **fixed** with the commit that fixed it (re-checked by the case named in the Case column).

| # | Case | What happens | Owner | Status |
| --- | --- | --- | --- | --- |
| 1 | LO-10 | The API accepted a guest phone with 8 digits; the rule is 9 to 11 digits with an optional 0 or +84. | API-2 | fixed d480325 |
| 2 | GD-07 | Offline on the payment screen showed no "no connection" state. | WEB-1 | fixed 168703e |
| 3 | TD-03 | `markAlertRead` answered 204 but the alert carried no read state. | API-2 | fixed b0f928e |
| 4 | TT-11, TT-17 | Outgoing money and money to another account appeared in Giao dịch as an unmatched transfer and could be linked to a bill. | API-1 | fixed 2b9b88f |
| 5 | TT-08 | The receipt showed one merged transfer line, and a paid bill kept a MISMATCH row that needed action. | API-2 | fixed b4dcb97 |
| 6 | GT-03 | A guest ID reveal was filed under RATES_SETTINGS, so the GUEST_ID filter found nothing. | API-1 | fixed 185a589 |
| 7 | DN-16 | A sign-in rate limit (429) was shown as "Account temporarily locked, the owner has been notified". | WEB-1 | fixed 996061a |
| 8 | CA-12 | The owner overview counted a float left in the drawer twice. | API-2 | fixed 8cbfbe1 |
| 9 | UI-01 | Raw `LOCK_ROOM` and other raw codes on Cảnh báo; `{room}` and an empty actor in Nhật ký. | WEB-2 | fixed f0666c8, e174d33 |
| 10 | (runner) | The production image did not build: web tests import `contracts/*.json`, which the Dockerfile left out. | WEB-2 | fixed 958f57f |
| 11 | UI-01 | `/owner/transactions` scrolls sideways at 390 px (page width 422 > 390, owner and vi). | WEB-2 | fixed 542aa4f |
| 12 | TT-15 | A QR never expires: `qrExpiryMinutes` is stored but nothing sets a pending payment to EXPIRED (only creating a new payment does), so the "QR expired" screen can never show. Docs/15 rule 2. | API-1 | open |
| 13 | CA-08 | The shift review does not say who the shift was handed to (`handoverToUserId` is accepted at close and never read back). The checklist expects "người bàn giao". | API-2 | open |

Observation (not a bug): owner cash for a stay goes on the latest-opened shift that can edit the building, not on the stay's own receptionist's shift (by design in `LockOpenShiftForStay`); with several shifts open the drawer can differ from the one the guest paid into.

Not bugs, but worth knowing: the shared compose project `stayguard-rehearse` means two sessions running `make rehearse-test` at once break each other
(the second `up --build` recreates the API under the first). The runner now takes a lock and refuses to start a second run.
