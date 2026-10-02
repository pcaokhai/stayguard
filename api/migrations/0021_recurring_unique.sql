-- +goose Up
-- One copy of a recurring expense per chain and month, enforced by the database whatever the code does.
CREATE UNIQUE INDEX expenses_one_copy_per_chain_month ON app.expenses (tenant_id, root_id, month)
    WHERE root_id IS NOT NULL AND source = 'RECURRING';
