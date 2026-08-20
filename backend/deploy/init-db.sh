#!/bin/bash
set -e

# Creates the dev database plus a separate test database used by integration tests,
# and the application role under which the services connect.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE cashx_test;
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cashx_app') THEN
            CREATE ROLE cashx_app LOGIN PASSWORD 'cashx_app_dev_password';
        END IF;
    END
    \$\$;
EOSQL
