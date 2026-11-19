CREATE TABLE puzzles (
    id                text PRIMARY KEY,   -- content-addressed, see puzzles.Publish
    body              jsonb NOT NULL,     -- PuzzleDefinition as received
    status            text NOT NULL CHECK (status IN ('pending', 'published', 'rejected')),
    verdict           jsonb,              -- validate.Verdict
    delete_token_hash bytea NOT NULL,
    owner_id          bigint,             -- set once accounts exist
    report_count      integer NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX puzzles_status_created ON puzzles (status, created_at DESC);
