-- +goose Up
-- create "google_connections" table
CREATE TABLE `google_connections` (`id` bigint NOT NULL AUTO_INCREMENT, `refresh_token` longtext NOT NULL, `access_token` longtext NOT NULL, `token_expiry` timestamp NULL, `calendar_id` varchar(255) NOT NULL DEFAULT "", `connected` bool NOT NULL DEFAULT 0, `created_at` timestamp NOT NULL, `updated_at` timestamp NOT NULL, PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_bin;
-- add mirror sync columns to "calendar_events" table
ALTER TABLE `calendar_events` ADD COLUMN `google_event_id` varchar(255) NOT NULL DEFAULT "", ADD COLUMN `sync_pending` bool NOT NULL DEFAULT 0;

-- +goose Down
-- reverse: add mirror sync columns to "calendar_events" table
ALTER TABLE `calendar_events` DROP COLUMN `sync_pending`, DROP COLUMN `google_event_id`;
-- reverse: create "google_connections" table
DROP TABLE `google_connections`;
