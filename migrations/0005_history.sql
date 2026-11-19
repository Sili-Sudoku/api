-- Opt-in solve history. One row per user and puzzle. No foreign key to puzzles: the id may be a bundled puzzle,
-- and a row survives the deletion of a published puzzle.
CREATE TABLE solve_history (
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    puzzle_id  text   NOT NULL CHECK (puzzle_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    solved_at  timestamptz,                                  -- when the best time was set, if the client knows
    seconds    integer CHECK (seconds BETWEEN 1 AND 8640000), -- the best solve time
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, puzzle_id)
);

CREATE INDEX solve_history_puzzle ON solve_history (puzzle_id) WHERE seconds IS NOT NULL;
CREATE INDEX solve_history_user_updated ON solve_history (user_id, updated_at DESC, puzzle_id);
