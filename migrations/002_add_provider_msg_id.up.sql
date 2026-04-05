ALTER TABLE notifications ADD COLUMN provider_msg_id TEXT;

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check
    CHECK (status IN ('pending', 'processing', 'delivered', 'failed', 'cancelled'));
