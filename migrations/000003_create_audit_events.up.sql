CREATE TABLE IF NOT EXISTS audit_events (
    id            BIGSERIAL PRIMARY KEY,
    event_type    TEXT        NOT NULL,
    user_id       BIGINT,
    email         TEXT        NOT NULL,
    ip            TEXT        NOT NULL,
    request_id    TEXT        NOT NULL,
    metadata      TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_type ON audit_events (event_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events (created_at);
