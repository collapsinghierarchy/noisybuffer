
CREATE TABLE IF NOT EXISTS apps (
    id      UUID PRIMARY KEY,
    name    TEXT NOT NULL,
    kid     SMALLINT    DEFAULT 0,   -- key version
    pubkey  BYTEA       -- nullable until the client uploads
);

CREATE TABLE IF NOT EXISTS submissions (
    id       UUID PRIMARY KEY,
    app_id   UUID       NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    kid      SMALLINT   NOT NULL,  -- matches uint8 in Go
    ts       TIMESTAMPTZ NOT NULL,
    blob     BYTEA      NOT NULL
);

CREATE INDEX IF NOT EXISTS submissions_app_ts_idx
    ON submissions (app_id, ts);