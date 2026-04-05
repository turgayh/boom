CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID,
    priority        TEXT        NOT NULL DEFAULT 'normal' CHECK (priority IN ('high', 'normal', 'low')),
    status          TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'cancelled')),
    idempotency_key TEXT        NOT NULL UNIQUE,
    recipient       TEXT        NOT NULL,
    channel         TEXT        NOT NULL CHECK (channel IN ('email', 'sms', 'push')),
    content         TEXT        NOT NULL,
    attempts        INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_status ON notifications (status);
CREATE INDEX idx_notifications_channel ON notifications (channel);
CREATE INDEX idx_notifications_batch_id ON notifications (batch_id);
