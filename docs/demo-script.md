# Demo script: StayGuard in 3 minutes

Audience: a guesthouse owner. Every step below uses behaviour that exists on `main`. Evidence column says where it is tested.
**Not yet run against the demo data**: `make demo-reset` is not on `main` yet, so account names and room codes below are placeholders (marked ⟨…⟩). Fill them in after it lands.

Legend: ✅ covered by an automated test (named) · ⚠️ built and used by the tests through the API, but I have not walked this screen in the running app for this script.

Prepare: `make demo-reset` (API-1, pending), open the app in `/vi` on a laptop, a phone (390 px) beside it if you have one. Roles: **RECEPTIONIST** ⟨name⟩, **OWNER** ⟨name⟩.

| Time | Sign in as | Do | Say | Check |
|---|---|---|---|---|
| 0:00 | Receptionist | Sign in with the guesthouse code, user name, PIN. Open the room map. | "Staff sign in with their own PIN. Five wrong tries lock the account." | ✅ `smoke.spec` (sign-in), `signInErrors.test` (429 and lock) |
| 0:20 | Receptionist | Tap a vacant room → Check in. Name, phone, overnight, default deposit. Optionally the ID number or a photo. | "The ID is optional and only the owner and managers can ever read it." | ✅ `smoke.spec` step 2; ID rules `rehearsal/guestid.spec` GT-02 |
| 0:45 | Receptionist | Add two bottles of water (extras). | "Prices come from the server, the screen never calculates them." | ✅ `smoke.spec` step 3 |
| 1:00 | Receptionist | Check out → **Pay by QR**. The QR names the bill code and the amount. | "The QR always pays the guesthouse's own account." | ✅ `smoke.spec` step 4 |
| 1:15 | (bank, not a person) | Pay the QR from a banking app or send the signed test webhook. The screen turns **Paid** by itself. | "Only the bank's report marks it paid. Nobody can tick it by hand." | ✅ `smoke.spec` step 5 (webhook); a real bank app: not verified |
| 1:30 | Receptionist | **Short transfer then top-up**: on another room, check out, pay by QR, send 10,000 less than the bill. The screen shows what arrived and what is left, with a QR for the remainder. Send the rest: **Paid**. | "A guest who sends too little just tops up. Same bill code, nothing lost." | ✅ `smoke.then-partial.spec`; leaving and returning: `smoke.then-awaiting.spec` |
| 1:55 | Receptionist | Open **Shift**. The cash taken line lists the deposit. Count the drawer, close the shift. | "Closing short needs a reason, and the owner is told." | ✅ `smoke.spec` step 7; short close `rehearsal/shift.spec` CA-04 (API level) · ⚠️ short-close wording on screen |
| 2:15 | Owner | Sign in on the laptop. **Overview**: building card with the day's revenue. | "The owner sees the money as it lands, on any device." | ✅ `smoke.spec` step 8 |
| 2:30 | Owner | **Alerts**: the open items (a short transfer or unpaid bill shows here and closes by itself when fixed). | "Problems come to you; they close when fixed." | ✅ `rehearsal/alerts.spec` TD-06 (API level) · ⚠️ alerts list UI not walked |
| 2:40 | Owner | **Activity log** (sidebar: Activity log): the check-in, payment, shift close. | "Every sensitive action is written and nobody can edit it." | ✅ log entries `rehearsal/guestid.spec` GT-03 (API) · ⚠️ page not walked |
| 2:48 | Owner | **Stays** → open the guest's stay → **Guest ID**: masked number, **Show** (hides again after a few seconds), photo. | "Owner and manager only. Every view is logged." | ✅ masking and reveal log GT-03 (API) · ⚠️ panel UI not walked |
| 2:56 | Owner | **Account** → language switch to English (also on the sign-in screen). Every screen follows. | "Vietnamese and English, per person." | ✅ `layout.spec` renders every route in vi and en · ⚠️ switch itself not walked here |

## If something goes wrong on stage

- Sign-in says "Too many attempts": wait the number of seconds shown, or use a second account.
- Transfer does not turn Paid: the polling is every 3 s; if the bank is slow, open the room again, it resumes the same bill code.
- Do not demo the guest ID with a real person's number; use a made-up one.

## Still to do

1. Replace the ⟨…⟩ placeholders with the `make demo-reset` accounts and rooms.
2. Walk every ⚠️ step in the running app and tick it here.
3. Screens (8 key screens, vi, 390 and 1280) and the payment GIF into `docs/assets/portfolio/`, after `make demo-reset` is on main.
