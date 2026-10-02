# Runbook: SePay handover for a new guesthouse

Place at `docs/runbooks/sepay-handover.md`. Referenced by docs/14 tasks P2 and P3.

Decision: there is no web page for tenant setup or SePay keys. The installer does it on the server over SSH (option A, 2026-10-02). An admin console is reconsidered at about 5 to 10 customers.

## Owner's part (with the owner, on the owner's phone, about 15 minutes)

1. The owner signs up to SePay in their own name (their phone number and ID). Never use the installer's account.
2. Link the receiving account that the QR will show.
3. Turn on incoming-money notifications.
4. Create the webhook together: incoming transactions, HMAC-SHA256, address from step 3 below. SePay shows the secret once.
5. The owner reads the secret aloud to the installer. Never send it by chat, SMS or email.
6. Check in StayGuard: Property and bank shows Connected.

## Server part (installer, SSH key only)

| Step | Command (names are proposals; built in P2 and P3) | Notes |
| --- | --- | --- |
| 1 | `ssh stayguard@[SERVER]` | Key-based SSH only |
| 2 | `stayguard tenant import --file <file>.json` | Creates buildings, rooms, rates, extras, receiving account and staff; prints one-time PINs (24 h). Keep the file out of git and delete it after import |
| 3 | `stayguard sepay webhook --tenant <CODE>` | Prints `https://[DOMAIN]/v1/webhooks/bank/<hookId>` |
| 4 | `stayguard sepay set-secret --tenant <CODE>` | Prompts with hidden input. Never pass the secret as an argument or environment variable. Stores it encrypted and writes an audit entry the owner sees: "Installer updated SePay connection" |
| 5 | `stayguard sepay status --tenant <CODE>` | Shows connection state, last webhook time, signature check |

Test: simulate an incoming transfer in SePay Test mode, then one real 2,000đ transfer in Live mode; the QR screen must turn Paid by itself.

## Rules

- The secret is write-only: no command or screen prints it back.
- Leaked secret: create a new one in SePay and repeat step 4.
- Owner changes bank account: redo the owner's part, then steps 3 to 5.
- Until connected, the front desk can take cash only; QR turns on after a successful test transaction.

## Lines to add to docs/14

- P2: "Also store `hookId` per tenant at import; the CLI must follow docs/runbooks/sepay-handover.md."
- P3: "Add the `stayguard sepay webhook | set-secret | status` subcommands; no HTTP endpoint or UI may read or write the secret."
