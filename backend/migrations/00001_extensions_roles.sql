-- +goose Up
-- Application role under which all CashX services connect.
-- Production bootstraps this role outside goose; the DO block is a no-op there.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cashx_app') THEN
        CREATE ROLE cashx_app LOGIN PASSWORD 'cashx_app_dev_password';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Role is intentionally not dropped (may be owned by objects in production).
