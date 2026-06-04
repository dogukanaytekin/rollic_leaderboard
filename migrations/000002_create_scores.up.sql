CREATE TABLE scores (
    id        BIGSERIAL   PRIMARY KEY,
    board_id  BIGINT      NOT NULL REFERENCES boards(id),
    user_id   TEXT        NOT NULL,
    score     BIGINT      NOT NULL,
    scored_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (board_id, user_id)
);

CREATE INDEX ON scores (board_id, score DESC, scored_at ASC);
