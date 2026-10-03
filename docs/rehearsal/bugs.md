# Rehearsal bugs (QA, portfolio demo)

Found by `make rehearse-test` (`docs/rehearsal/results-<date>.csv`, evidence under `evidence-<date>/<id>/`). Owner is the lane that fixes it.
Status: **open**, or **fixed** with the commit that fixed it (re-checked by the case named in the Case column).

| # | Case | What happens | Owner | Status |
| --- | --- | --- | --- | --- |
| 1 | LO-10 | The API accepts a guest phone with 8 digits (`01234567`); the rule is 9 to 11 digits starting with 0 or +84 (the form already refuses it, the API must too). | API-2 | open |
| 2 | GD-07 | Offline on the payment screen: the screen shows no "no connection" state after two missed polls (3 s each). Coming back creates no second payment (that part passes). | WEB-1 | open |
| 3 | TD-03 | `markAlertRead` answers 204, but the alert in `listAlerts` carries no read state (the schema has no field for it), so the API cannot say what the bell count is made of. Money and stay-time alerts do stay on the list. | API-2 | open |
| 4 | TT-11, TT-17 | Outgoing money and money to another account appeared in Giao dịch as an unmatched transfer and could be linked to a bill. | API-1 | fixed 2b9b88f |
| 5 | TT-08 | The receipt showed one merged transfer line, and a paid bill kept a MISMATCH row that needed action. | API-2 | fixed b4dcb97 |
| 6 | GT-03 | A guest ID reveal was filed under RATES_SETTINGS, so the GUEST_ID filter found nothing. | API-1 | fixed 185a589 |
| 7 | DN-16 | A sign-in rate limit (429) was shown as "Account temporarily locked, the owner has been notified". | WEB-1 | fixed 996061a |
| 8 | CA-12 | The owner overview counted a float left in the drawer twice. | API-2 | fixed 8cbfbe1 |
| 9 | UI-01 | Raw `LOCK_ROOM` and other raw codes on Cảnh báo; `{room}` and an empty actor in Nhật ký. | WEB-2 | fixed f0666c8, e174d33 |
| 10 | (runner) | The production image did not build: web tests import `contracts/*.json`, which the Dockerfile left out. | WEB-2 | fixed 958f57f |

Not bugs, but worth knowing: the shared compose project `stayguard-rehearse` means two sessions running `make rehearse-test` at once break each other
(the second `up --build` recreates the API under the first). The runner now takes a lock and refuses to start a second run.
