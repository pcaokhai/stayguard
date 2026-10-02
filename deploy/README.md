# deploy

Production runs on one VPS with Docker: PostgreSQL, the API (which also serves the web app) and Caddy for automatic HTTPS.
The files: `compose.prod.yaml`, `Caddyfile`, `.env.prod.example`, `backup.sh`, `restore-rehearsal.sh`. (`compose.yaml` and
`compose.demo.override.yaml` are for local use only.)

What the production stack guarantees: `DEMO_MODE=0`; the API connects as `stayguard_app`, a non-superuser role that owns
nothing and cannot bypass row-level security (`ALLOW_PRIVILEGED_DB` is never set for it); the database port is not
published; every service restarts unless stopped; `TRUST_PROXY=1` and `TRUSTED_PROXY_HOPS=1` with Caddy as the only way in.

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
8. **Check it**: `curl -fsS https://DOMAIN/readyz` answers 200; open `https://DOMAIN/vi/` and see the sign-in page without the
   demo role picker. `docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod ps` shows `db`, `api` and `caddy` up.
   If HTTPS fails, `docker compose ... logs caddy`: the usual cause is DNS not pointing at the server yet.
9. **Add the first guesthouse** (docs/runbooks/sepay-handover.md has the SePay part). Put the import file in `/tmp`, outside git:
   ```
   docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod run --rm -v /tmp/tenant.json:/tmp/tenant.json:ro api tenant import --file /tmp/tenant.json
   shred -u /tmp/tenant.json
   ```
   Then `... run --rm api sepay webhook --tenant CODE`, `... exec api sepay set-secret --tenant CODE` (hidden prompt) and
   `... exec api sepay status --tenant CODE`. Give staff their one-time PINs.
10. **Daily jobs** (guest ID retention, recurring expenses): one cron line for the `stayguard` user, `crontab -e`:
    ```
    15 3 * * * cd /srv/stayguard && docker compose -f deploy/compose.prod.yaml --env-file deploy/.env.prod exec -T api /app/stayguard jobs run >> /var/log/stayguard-jobs.log 2>&1
    ```
    Make the log writable once: `sudo touch /var/log/stayguard-jobs.log && sudo chown stayguard: /var/log/stayguard-jobs.log`.
11. **Backups**: set up an rclone remote on a different provider than the server (`rclone config`; make it a `crypt` remote so the
    dumps are encrypted), put its name in `BACKUP_REMOTE` in `deploy/.env.prod`, then:
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
