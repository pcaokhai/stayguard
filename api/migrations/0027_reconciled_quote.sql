-- +goose Up
-- The deposit is a fact of the stay (taken in cash at check-in). While an invoice is open its quote must show that deposit, so a quote
-- frozen with another figure is read as total minus the stay's real deposit. A paid invoice keeps its history as it was.
-- +goose StatementBegin
CREATE FUNCTION app.reconciled_quote(quote jsonb, total bigint, status text, deposit bigint) RETURNS jsonb
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE WHEN status = 'OPEN' THEN
        quote || jsonb_build_object('depositPaid', deposit, 'balanceDue', greatest(total - deposit, 0), 'refundDue', greatest(deposit - total, 0))
        ELSE quote END
$$;
-- +goose StatementEnd

UPDATE app.invoices i SET quote = app.reconciled_quote(i.quote, i.total, i.status, s.deposit)
FROM app.stays s
WHERE s.tenant_id = i.tenant_id AND s.id = i.stay_id AND i.status = 'OPEN' AND i.quote IS DISTINCT FROM app.reconciled_quote(i.quote, i.total, i.status, s.deposit);
