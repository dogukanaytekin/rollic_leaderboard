CREATE TABLE boards (
    id               BIGSERIAL   PRIMARY KEY,
    name             TEXT        NOT NULL,
    description      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    schedule_type    TEXT,
    interval_seconds INTEGER,

    CHECK (
        (schedule_type IS NULL AND interval_seconds IS NULL)
        OR (schedule_type = 'interval' AND interval_seconds > 0)
    )
);