-- Draft schema (Postgres). Not applied by anything yet.
-- Order of work: puzzles first (anonymous), then users, then everything that points at a user.

CREATE TABLE puzzles (
    id           text PRIMARY KEY,               -- content-addressed, see puzzles.Publish
    body         jsonb NOT NULL,                 -- PuzzleDefinition as received
    status       text NOT NULL CHECK (status IN ('pending', 'published', 'rejected')),
    verdict      jsonb,                          -- validate.Verdict
    delete_token_hash bytea NOT NULL,
    owner_id     bigint,                         -- set once accounts exist
    report_count integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- users and sessions moved to ../0002_accounts.sql

CREATE TABLE ratings (
    puzzle_id  text NOT NULL REFERENCES puzzles(id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stars      smallint CHECK (stars BETWEEN 1 AND 5),
    difficulty smallint CHECK (difficulty BETWEEN 1 AND 5),
    PRIMARY KEY (puzzle_id, user_id)
);

CREATE TABLE comments (
    id         bigserial PRIMARY KEY,
    puzzle_id  text NOT NULL REFERENCES puzzles(id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       text NOT NULL CHECK (length(body) <= 2000), -- plain text, never HTML
    hidden     boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE follows (
    follower_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creator_id  bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (follower_id, creator_id)
);

CREATE TABLE solve_history (
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    puzzle_id  text NOT NULL,                    -- no FK: history survives a puzzle being deleted
    solved_at  timestamptz,
    seconds    integer,
    progress   jsonb,                            -- versioned blob, same shape as the web app's localStorage
    PRIMARY KEY (user_id, puzzle_id)
);
