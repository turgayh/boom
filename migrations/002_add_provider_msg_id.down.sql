ALTER TABLE notifications DROP COLUMN IF EXISTS provider_msg_id;

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check
    CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'cancelled'));
