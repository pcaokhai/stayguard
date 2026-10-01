-- name: InsertAuditLog :exec
INSERT INTO app.audit_logs (id, tenant_id, actor_id, action, entity_type, entity_id, before, after, trace_id)
VALUES (@id, @tenant_id, sqlc.narg(actor_id), @action, @entity_type, @entity_id, @before, @after, sqlc.narg(trace_id));
