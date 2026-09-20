-- +goose Up
-- add "llm_provider" column to "app_settings" table
ALTER TABLE `app_settings` ADD COLUMN `llm_provider` enum('deepseek','claude') NOT NULL DEFAULT "deepseek";

-- +goose Down
-- reverse: add "llm_provider" column to "app_settings" table
ALTER TABLE `app_settings` DROP COLUMN `llm_provider`;
