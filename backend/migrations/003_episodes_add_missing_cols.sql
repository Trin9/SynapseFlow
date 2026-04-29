-- M1.0.1: Add missing Episode columns omitted from 002_episodes.sql.
-- These columns correspond to Episode schema v1.0 fields that were present in
-- models.go but absent from the DB table, causing silent data loss on every
-- Create/Update call in Postgres mode.

ALTER TABLE episodes
    ADD COLUMN IF NOT EXISTS status                TEXT        NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS trigger               JSONB       NULL,
    ADD COLUMN IF NOT EXISTS action_context        JSONB       NULL,
    ADD COLUMN IF NOT EXISTS investigation_context JSONB       NULL,
    ADD COLUMN IF NOT EXISTS memory_extraction     JSONB       NULL,
    ADD COLUMN IF NOT EXISTS concluded_at          TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS human_interventions   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS parent_episode_id     TEXT        NULL;
