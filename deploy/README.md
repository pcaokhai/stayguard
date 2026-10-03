# deploy

Production runs on one VPS with Docker: PostgreSQL, the API (which also serves the web app) and Caddy for automatic HTTPS.
The files: `compose.prod.yaml`, `Caddyfile`, `.env.prod.example`, `backup.sh`, `restore-rehearsal.sh`. (`compose.yaml` and
`compose.demo.override.yaml`, `compose.smoke.override.yaml` and `compose.rehearse.yaml` are for local use only; `compose.yaml` also has a local S3 test target under profile `backup`.)

What the production stack guarantees (with `DEMO_MODE` left at its default `0`; see the demo section below); the API connects as `stayguard_app`, a non-superuser role that owns
nothing and cannot bypass row-level security (`ALLOW_PRIVILEGED_DB` is never set for it); the database port is not
published; every service restarts unless stopped; `TRUST_PROXY=1` and `TRUSTED_PROXY_HOPS=1` with Caddy as the only way in.

## Demo build and the role picker

This repository is a portfolio demo, so the web image is built **with** the demo role picker by default: `compose.prod.yaml` passes
`NEXT_PUBLIC_DEMO_MODE=${NEXT_PUBLIC_DEMO_MODE:-1}`. The picker only works when the API also runs with `DEMO_MODE=1` (default `0`, which answers
the demo routes with 404). Set both in `deploy/.env.prod` for a demo server:

```
NEXT_PUBLIC_DEMO_MODE=1
DEMO_MODE=1
```

For a build with the real PIN sign-in only (no picker), build with `0`, either in `deploy/.env.prod` or on the command line, and keep `DEMO_MODE=0`:

```
NEXT_PUBLIC_DEMO_MODE=0 docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod up -d --build
```

The value is baked into the web bundle at build time, so changing it needs `--build`. The rehearsal stack (`compose.rehearse.yaml`) defaults to `0` because
it imitates production. CI builds the image with `0` (`image` job). `make demo-check` runs the whole demo gate.

## Fresh VPS, step by step

Ubuntu 24.04 LTS, 2 GB RAM or more, a domain whose DNS A record points at the server (do this first, HTTPS needs it).

1. **Log in and update**: `ssh root@SERVER`, then `apt update && apt -y upgrade`.
2. **Create a deploy user with your SSH key**, and turn off password login:
   ```
   adduser --disabled-password --gecos "" stayguard && usermod -aG sudo stayguard
   mkdir -p /home/stayguard/.ssh && cp ~/.ssh/authorized_keys /home/stayguard/.ssh/ && chown -R stayguard: /home/stayguard/.ssh
   sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && systemctl reload ssh
   ```
   Open a second terminal and check `ssh stayguard@SERVER` works before closing the first.
3. **Firewall**: only SSH, HTTP and HTTPS.
   ```
   ufw allow OpenSSH && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable
   ```
4. **Install Docker, Git and rclone**: `curl -fsSL https://get.docker.com | sh && usermod -aG docker stayguard && apt -y install git rclone`.
   Log in again as `stayguard`.
5. **Get the code**: `sudo mkdir -p /srv/stayguard && sudo chown stayguard: /srv/stayguard && git clone REPO_URL /srv/stayguard && cd /srv/stayguard`.
6. **Write the secrets file** (it never goes into git):
   ```
   cp deploy/.env.prod.example deploy/.env.prod && chmod 600 deploy/.env.prod
   ```
   Fill in `DOMAIN`, `ACME_EMAIL`, and the three secrets with the commands in the comments of the file
   (`POSTGRES_PASSWORD`, `APP_DB_PASSWORD`, `DATA_ENCRYPTION_KEY`). **Copy `DATA_ENCRYPTION_KEY` to a password manager now**:
   without it guest ID data and bank accounts cannot be read, and a backup cannot bring them back.
7. **Start it** (the first build takes a few minutes):
   ```
   docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod up -d --build
   ```
   The `migrate` and `roles` containers run once and exit; that is normal.
8. **Check it**: `curl -fsS https://DOMAIN/readyz` answers 200; open `https://DOMAIN/vi/` and see the sign-in page (without the demo role picker when built with `NEXT_PUBLIC_DEMO_MODE=0`). `docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod ps` shows `db`, `api`, `jobs` and `caddy` up.
   If HTTPS fails, `docker compose ... logs caddy`: the usual cause is DNS not pointing at the server yet.
9. **Add the first guesthouse** (docs/runbooks/sepay-handover.md has the SePay part). Put the import file in `/tmp`, outside git:
   ```
   docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod run --rm -v /tmp/tenant.json:/tmp/tenant.json:ro api tenant import --file /tmp/tenant.json
   shred -u /tmp/tenant.json
   ```
   Then `... run --rm api sepay webhook --tenant CODE`, `... exec api sepay set-secret --tenant CODE` (hidden prompt) and
   `... exec api sepay status --tenant CODE`. Give staff their one-time PINs.
10. **Jobs**: nothing to set up. The `jobs` service in `compose.prod.yaml` runs `stayguard jobs run --every 5m` (partial-transfer alerts
    after 15 minutes, guest ID retention, recurring expenses) with the app's environment, restarts unless stopped, and is a single
    instance: do not scale it. Check it with `docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod logs jobs`;
    each round prints one count line per job, and a failed round says so and the next one still runs. No cron entry is needed.
11. **Backups**: the dumps go to S3-compatible storage at a different provider than the server (see "Backup storage" below). Put the
    `BACKUP_S3_*` settings in `deploy/.env.prod`, then:
    ```
    sudo mkdir -p /var/backups/stayguard && sudo chown stayguard: /var/backups/stayguard
    deploy/backup.sh                       # run it once by hand and read the last line: "backup ok"
    (crontab -l; echo '30 2 * * * cd /srv/stayguard && deploy/backup.sh >> /var/log/stayguard-backup.log 2>&1') | crontab -
    ```
12. **Rehearse the restore now, and once a month**: `deploy/restore-rehearsal.sh` restores the newest remote dump into a scratch
    database, checks it and drops it. It must end with `restore rehearsal ok`. The first time, write the date of this check in
    your notes; no rehearsal, no backup.
13. **Test the money path** before the first real guest: SePay Test-mode transfer, then one real 2,000 d transfer; the QR must turn
    Paid by itself (see the runbook).

## Rehearsal on your own machine (SePay and Cloudflare tunnel)

`make rehearse` runs this same production stack locally before you touch a VPS: `compose.prod.yaml` plus `compose.rehearse.yaml`,
`DEMO_MODE` off, the non-superuser database role, and plain HTTP on `127.0.0.1:18090` (Caddy is left out; set
`REHEARSE_PORT` to change the port). It imports a fake guesthouse (`rehearse`) with the installer commands, sets the SePay secret, and
prints once: the owner and receptionist one-time PINs, the exact webhook path, the secret and the tunnel command. It starts only the api (which serves the web app), PostgreSQL and the `jobs` service, and pulls nothing from quay.io. With
`REHEARSE_BACKUP=1` it also takes one backup to rclone's built-in S3 server (`backup ok`). Needs docker, python3 and openssl; the first build takes a few minutes.

```
make rehearse                                  # prints the PINs and the webhook path once
cloudflared tunnel --url http://localhost:18090  # a public https address for SePay to call
```

In SePay set the webhook URL to `https://<tunnel-host>` plus the printed path, and the secret to the printed one (or start with
`SEPAY_SECRET=...` to use yours). For a real transfer to turn Paid, start with your SePay receiving account:
`REHEARSE_BANK_BIN=970436 REHEARSE_ACCOUNT_NO=... REHEARSE_ACCOUNT_NAME=... make rehearse`. The secrets live in `deploy/.env.rehearse`
(git-ignored) so a restart keeps the key that matches the data. Running it again after the guesthouse exists stops at the import:
`make rehearse-down` deletes the containers, volumes, bucket, secrets file and local dumps, and the next `make rehearse` starts clean.
Never put real guest data in it. To rehearse the backup against a real bucket, set the `BACKUP_S3_*` variables for it and run
`REHEARSE_BACKUP=1 EXTERNAL_S3=1 make rehearse`.

## Backup storage (S3-compatible)

`backup.sh` and `restore-rehearsal.sh` need an S3-compatible bucket: AWS S3, Cloudflare R2, Backblaze B2, Wasabi. Create a
bucket and an access key limited to it, and set in `deploy/.env.prod` (placeholders are in `deploy/.env.example`):

```
BACKUP_S3_ENDPOINT=https://s3.eu-central-003.backblazeb2.com   # the provider's S3 endpoint
BACKUP_S3_BUCKET=stayguard-backups
BACKUP_S3_ACCESS_KEY=...
BACKUP_S3_SECRET_KEY=...
BACKUP_S3_PROVIDER=Other        # AWS for AWS S3, Cloudflare for R2, Other for B2, Wasabi
BACKUP_S3_REGION=               # AWS only, e.g. eu-central-1
BACKUP_S3_PREFIX=               # optional folder inside the bucket
```

The scripts turn these into an rclone remote named `s3backup` through rclone's environment configuration, so there is no config
file to keep. rclone does not need to be installed: when it is missing the scripts run the official `rclone/rclone` Docker image
with the same settings. Prefer a bucket with versioning or object lock so a mistake or an intruder cannot delete old backups.
The dump holds guest names and phones in the clear, so keep the bucket private. To encrypt it as well, configure an rclone
`crypt` remote yourself (`rclone config`) and set `BACKUP_REMOTE=thatremote:folder` instead of the `BACKUP_S3_*` settings.

**Try it locally first:** `make backup-test` starts a throwaway database and rclone's built-in S3 server (`rclone serve s3`, the official
`rclone/rclone` image from Docker Hub; compose profile `backup`), imports a test guesthouse, takes a backup, lists it, restores it into a
scratch database and checks that every table has the same row count. Nothing is pulled from quay.io.

**Test a real bucket** (Cloudflare R2, Backblaze B2, AWS S3) with the same rclone configuration the server will use: put its
`BACKUP_S3_*` settings in the environment (endpoint, bucket, keys, `BACKUP_S3_PROVIDER`; the bucket must exist) and run
`EXTERNAL_S3=1 make backup-test`, or `REHEARSE_BACKUP=1 EXTERNAL_S3=1 make rehearse` for the production stack.

## Update and roll back

```
cd /srv/stayguard && git pull && docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod up -d --build
```
Migrations run again on start (forward only). Take `deploy/backup.sh` first. To roll back code, `git checkout PREVIOUS_TAG` and
run the same command; a migration that already ran is not undone, so restore from backup only if data was damaged.

## Reverse proxy and the client address (sign-in rate limits)

Sign-in is rate limited per client address. Behind the reverse proxy the address comes from `X-Forwarded-For`, so the
production environment (`deploy/.env.prod`) sets:

```
TRUST_PROXY=1
TRUSTED_PROXY_HOPS=1
```

**Assumption: exactly one proxy, Caddy, stands in front of the server and the server port is not reachable any other way.**
Caddy's `reverse_proxy` replaces a client-supplied `X-Forwarded-For` (unless `trusted_proxies` is set) and appends the real
peer, so the last entry is the client. `TRUSTED_PROXY_HOPS` is how many entries from the right the client address is: add a
second proxy layer (a CDN or load balancer in front of Caddy) and set it to 2, and tell Caddy to trust that layer.

With `TRUST_PROXY` off (the default, and the local `make up`) the TCP peer address is used. Never set `TRUST_PROXY=1` when
the server is reachable without the proxy: a client could then choose its own address and escape the per-IP limit. The
lockout after five wrong PINs lives in the database per account and does not depend on this setting.
