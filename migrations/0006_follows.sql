-- A follower sees the puzzles of the creators they follow in a feed.
CREATE TABLE follows (
    follower_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creator_id  bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, creator_id),
    CHECK (follower_id <> creator_id)
);

CREATE INDEX follows_creator ON follows (creator_id);
