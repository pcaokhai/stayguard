-- +goose Up
-- /readyz runs as the application role and must read which migrations are applied (read-only; it can still not alter them).
GRANT SELECT ON public.goose_db_version TO stayguard_app;
