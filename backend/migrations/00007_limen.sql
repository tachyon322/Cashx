-- +goose Up
-- Switch authentication to the Limen library: password hashes move from
-- password_credentials into users.password (Argon2id, managed by Limen),
-- sessions/verification_tokens are replaced by Limen's sessions/verifications.
ALTER TABLE users
    ADD COLUMN password varchar(255),
    ADD COLUMN email_verified_at timestamptz;

DROP TABLE password_credentials;
DROP TABLE verification_tokens;
DROP TABLE sessions;

-- Exact Limen schema (sessions): id/token/user_id/created_at/expires_at/
-- last_access/metadata. id stays uuid via the global ID generator; the column
-- DEFAULT covers any path where Limen does not fill id itself.
CREATE TABLE sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token       varchar(255) NOT NULL UNIQUE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    last_access timestamptz NOT NULL,
    metadata    jsonb
);

-- Exact Limen schema (verifications): used for password resets.
CREATE TABLE verifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject    varchar(255) NOT NULL,
    value      varchar(255) NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX verifications_subject_idx ON verifications(subject);

-- Application role grants for the new Limen tables (grants on the dropped
-- tables died with them). DELETE is required: Limen deletes session rows on
-- signout and consumed verification rows on password reset.
GRANT SELECT, INSERT, UPDATE, DELETE ON sessions, verifications TO cashx_app;

-- +goose Down
DROP TABLE verifications;
DROP TABLE sessions;

ALTER TABLE users
    DROP COLUMN password,
    DROP COLUMN email_verified_at;

CREATE TABLE verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('email_verification', 'password_reset')),
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX verification_tokens_user_id_idx ON verification_tokens(user_id);

CREATE TABLE password_credentials (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   text NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    ip           inet,
    user_agent   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);

-- Restore the app-role grants that 00006 issued for the recreated tables.
GRANT SELECT, INSERT, UPDATE ON sessions, password_credentials, verification_tokens TO cashx_app;
