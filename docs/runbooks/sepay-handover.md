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

## Server settings the installer must check once

- `TRUST_PROXY=1` and `TRUSTED_PROXY_HOPS=1` in `deploy/.env.prod`: one Caddy in front, the app port closed to the internet (see `deploy/README.md`). Without them every sign-in shares the proxy's address and the per-IP limit locks everyone out together.

- Daily jobs (guest ID retention today; more later): one cron line on the server, with `DATABASE_URL` and `DATA_ENCRYPTION_KEY` as the app uses (no other database login):

  ```
  */5 * * * * stayguard cd /srv/stayguard && docker compose --env-file deploy/.env.prod exec -T api stayguard jobs run >> /var/log/stayguard-jobs.log 2>&1
  ```

  It prints counts only. Check the first night's run in the log.

## If SePay deliveries are rejected for age

SePay's signature covers a timestamp and the server accepts it within `SEPAY_TIMESTAMP_TOLERANCE` of its own clock (default `300s`, the window SePay documents). The log line `sepay webhook rejected: timestamp outside tolerance` carries a running count (`rejected_for_age_total`), the tenant and the skew in seconds, never the payload. If it appears for real transfers: first check the server clock (`timedatectl`, NTP), then widen the window in `deploy/.env.prod`, for example `SEPAY_TIMESTAMP_TOLERANCE=900s` (up to `24h`), and restart the API. A wider window only lets an old signed delivery be replayed for longer: duplicates are still dropped, because every event is deduplicated on SePay's transaction id.

## The data encryption key (back it up separately, never rotate it casually)

`DATA_ENCRYPTION_KEY` protects PIN hashes (as a pepper), guest ID data, bank accounts and the SePay secret. Keep a copy of it **apart from the database backups** (a password manager, not the backup bucket). If the database is restored or reused with a different key, every PIN looks wrong (PIN_INVALID) and encrypted data cannot be read. The server stores a fingerprint of the key on first start and refuses to boot against a different one with `key fingerprint mismatch`: restore the original key. Never rotate the key without a re-encryption job (none exists yet).

## Rules

- The secret is write-only: no command or screen prints it back.
- Leaked secret: create a new one in SePay and repeat step 4.
- Owner changes bank account: redo the owner's part, then steps 3 to 5.
- Until connected, the front desk can take cash only; QR turns on after a successful test transaction.

## Lines to add to docs/14

- P2: "Also store `hookId` per tenant at import; the CLI must follow docs/runbooks/sepay-handover.md."
- P3: "Add the `stayguard sepay webhook | set-secret | status` subcommands; no HTTP endpoint or UI may read or write the secret."
