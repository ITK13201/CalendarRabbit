-- +goose Up
-- create "app_settings" table
CREATE TABLE `app_settings` (`id` bigint NOT NULL AUTO_INCREMENT, `timezone` varchar(255) NOT NULL DEFAULT "Asia/Tokyo", `updated_at` timestamp NOT NULL, PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_bin;
-- create "calendar_events" table
CREATE TABLE `calendar_events` (`id` bigint NOT NULL AUTO_INCREMENT, `title` varchar(255) NOT NULL, `starts_at` timestamp NOT NULL, `ends_at` timestamp NOT NULL, `all_day` bool NOT NULL DEFAULT 0, `location` varchar(255) NOT NULL DEFAULT "", `description` longtext NOT NULL, `source_url` varchar(255) NOT NULL DEFAULT "", `created_at` timestamp NOT NULL, `updated_at` timestamp NOT NULL, PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_bin;
-- create "conversations" table
CREATE TABLE `conversations` (`id` bigint NOT NULL AUTO_INCREMENT, `created_at` timestamp NOT NULL, `updated_at` timestamp NOT NULL, PRIMARY KEY (`id`)) CHARSET utf8mb4 COLLATE utf8mb4_bin;
-- create "messages" table
CREATE TABLE `messages` (`id` bigint NOT NULL AUTO_INCREMENT, `role` enum('user','assistant') NOT NULL, `content` longtext NOT NULL, `created_at` timestamp NOT NULL, `conversation_messages` bigint NOT NULL, PRIMARY KEY (`id`), INDEX `messages_conversations_messages` (`conversation_messages`), CONSTRAINT `messages_conversations_messages` FOREIGN KEY (`conversation_messages`) REFERENCES `conversations` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION) CHARSET utf8mb4 COLLATE utf8mb4_bin;
-- create "event_proposals" table
CREATE TABLE `event_proposals` (`id` bigint NOT NULL AUTO_INCREMENT, `title` varchar(255) NOT NULL, `starts_at` timestamp NOT NULL, `ends_at` timestamp NOT NULL, `all_day` bool NOT NULL DEFAULT 0, `location` varchar(255) NOT NULL DEFAULT "", `description` longtext NOT NULL, `source_url` varchar(255) NOT NULL DEFAULT "", `status` enum('pending','approved','rejected') NOT NULL DEFAULT "pending", `created_at` timestamp NOT NULL, `updated_at` timestamp NOT NULL, `conversation_proposals` bigint NOT NULL, `event_proposal_calendar_event` bigint NULL, `message_proposals` bigint NULL, PRIMARY KEY (`id`), INDEX `event_proposals_calendar_events_calendar_event` (`event_proposal_calendar_event`), INDEX `event_proposals_conversations_proposals` (`conversation_proposals`), INDEX `event_proposals_messages_proposals` (`message_proposals`), CONSTRAINT `event_proposals_calendar_events_calendar_event` FOREIGN KEY (`event_proposal_calendar_event`) REFERENCES `calendar_events` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `event_proposals_conversations_proposals` FOREIGN KEY (`conversation_proposals`) REFERENCES `conversations` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `event_proposals_messages_proposals` FOREIGN KEY (`message_proposals`) REFERENCES `messages` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL) CHARSET utf8mb4 COLLATE utf8mb4_bin;

-- +goose Down
-- reverse: create "event_proposals" table
DROP TABLE `event_proposals`;
-- reverse: create "messages" table
DROP TABLE `messages`;
-- reverse: create "conversations" table
DROP TABLE `conversations`;
-- reverse: create "calendar_events" table
DROP TABLE `calendar_events`;
-- reverse: create "app_settings" table
DROP TABLE `app_settings`;
