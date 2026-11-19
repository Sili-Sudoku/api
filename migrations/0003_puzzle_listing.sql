-- Fields copied out of the puzzle body so lists can filter and sort without reading every body.
ALTER TABLE puzzles
    ADD COLUMN title      text   NOT NULL DEFAULT '',
    ADD COLUMN author     text   NOT NULL DEFAULT '',   -- author credit, plain text
    ADD COLUMN source     text   NOT NULL DEFAULT '',   -- source URL, plain text
    ADD COLUMN difficulty text   NOT NULL DEFAULT '',
    ADD COLUMN tags       text[] NOT NULL DEFAULT '{}',
    ADD COLUMN variants   text[] NOT NULL DEFAULT '{}', -- definitions the puzzle uses, for example "killer"
    -- none: shown unless it has 3 reports. hidden: an admin hid it. cleared: an admin reviewed it, reports no longer hide it.
    ADD COLUMN moderation text   NOT NULL DEFAULT 'none' CHECK (moderation IN ('none', 'hidden', 'cleared'));

CREATE INDEX puzzles_tags ON puzzles USING gin (tags);
CREATE INDEX puzzles_variants ON puzzles USING gin (variants);
CREATE INDEX puzzles_owner ON puzzles (owner_id);

-- One report per user and target. kind is "puzzle" or "comment".
CREATE TABLE reports (
    kind       text   NOT NULL CHECK (kind IN ('puzzle', 'comment')),
    target     text   NOT NULL,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, target, user_id)
);
