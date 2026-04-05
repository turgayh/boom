DROP INDEX IF EXISTS idx_notifications_status;
DROP INDEX IF EXISTS idx_notifications_channel;
DROP INDEX IF EXISTS idx_notifications_batch_id;
DROP INDEX IF EXISTS idx_notification_batches_status;
DROP INDEX IF EXISTS idx_notification_batches_notification_id;
DROP TABLE IF EXISTS notification_batches;
DROP TABLE IF EXISTS notifications;