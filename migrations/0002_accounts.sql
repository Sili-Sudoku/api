CREATE TABLE users (
    id            bigserial PRIMARY KEY,
    username      text NOT NULL CHECK (username ~ '^[A-Za-z0-9_]{3,24}$'),
    email         text CHECK (length(email) <= 254),
    password_hash text NOT NULL,
    bio           text NOT NULL DEFAULT '' CHECK (length(bio) <= 500),
    is_admin      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Names are unique ignoring case, so "Ann" and "ann" cannot both exist.
CREATE UNIQUE INDEX users_username_lower ON users (lower(username));

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,           -- sha256 of the cookie value
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user ON sessions (user_id);
CREATE INDEX sessions_expires ON sessions (expires_at);

-- A deleted account leaves its puzzles without an owner, unless the user chose to delete them too.
ALTER TABLE puzzles ADD CONSTRAINT puzzles_owner_fk FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;
