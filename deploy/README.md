# deploy

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
