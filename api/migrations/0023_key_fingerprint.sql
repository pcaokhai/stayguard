-- +goose Up
-- The data encryption key protects PIN hashes (as the pepper), guest ID data, bank accounts and the SePay secret. If the
-- key changes while the database lives on, all of that becomes unreadable and sign-in answers PIN_INVALID. The server
-- therefore stores a fingerprint of the key on first start and refuses to boot against a different key.
-- One row, no tenant: it holds a keyed digest that reveals nothing without the key. The application role may read it and
-- write it once; it cannot change or delete it.
CREATE TABLE public.key_fingerprint (
    id          boolean PRIMARY KEY DEFAULT true CHECK (id),
    fingerprint text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT, INSERT ON public.key_fingerprint TO stayguard_app;
