-- name: BeginIdempotencyKey :one
-- Claims the key, or takes over an expired one. No row back means a live key exists: the caller reads it.
-- The insert waits on the primary key while another transaction holds the same key, so only one proceeds.
INSERT INTO app.idempotency_keys (tenant_id, route, key, request_hash, expires_at)
VALUES (@tenant_id, @route, @key, @request_hash, now() + make_interval(secs => @ttl_seconds::float8))
ON CONFLICT (tenant_id, route, key) DO UPDATE
    SET request_hash = EXCLUDED.request_hash, status_code = NULL, response_body = NULL,
        created_at = now(), expires_at = EXCLUDED.expires_at
    WHERE app.idempotency_keys.expires_at <= now()
RETURNING key;

-- name: GetIdempotencyKey :one
SELECT request_hash, status_code, response_body
FROM app.idempotency_keys
WHERE tenant_id = @tenant_id AND route = @route AND key = @key;

-- name: CompleteIdempotencyKey :execrows
UPDATE app.idempotency_keys
SET status_code = @status_code, response_body = @response_body
WHERE tenant_id = @tenant_id AND route = @route AND key = @key;
