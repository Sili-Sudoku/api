-- Ratings, difficulty feedback and comments. One rating row per user and puzzle.
CREATE TABLE ratings (
    puzzle_id  text   NOT NULL REFERENCES puzzles(id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stars      smallint CHECK (stars BETWEEN 1 AND 5),       -- how good the puzzle is
    difficulty smallint CHECK (difficulty BETWEEN 1 AND 5),  -- how hard it felt
    PRIMARY KEY (puzzle_id, user_id)
);

-- Sums and counts of the rows above, kept on the puzzle so lists can sort by rating without a join.
-- The service recomputes them from ratings after each change, so a drift heals itself.
ALTER TABLE puzzles
    ADD COLUMN rating_count integer NOT NULL DEFAULT 0,
    ADD COLUMN rating_sum   integer NOT NULL DEFAULT 0,
    ADD COLUMN diff_count   integer NOT NULL DEFAULT 0,
    ADD COLUMN diff_sum     integer NOT NULL DEFAULT 0;

CREATE TABLE comments (
    id           bigserial PRIMARY KEY,
    puzzle_id    text   NOT NULL REFERENCES puzzles(id) ON DELETE CASCADE,
    user_id      bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body         text   NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000), -- plain text, never HTML
    report_count integer NOT NULL DEFAULT 0,
    moderation   text   NOT NULL DEFAULT 'none' CHECK (moderation IN ('none', 'hidden', 'cleared')),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_puzzle ON comments (puzzle_id, created_at DESC, id DESC);
