-- M5.2: Add execution_id to experiences so that memory recall results can surface
-- the originating execution to the frontend (SourceExecutionID field in MemoryRecallView).
-- Existing rows get an empty string default; new writes will carry the real execution ID.
ALTER TABLE experiences
    ADD COLUMN IF NOT EXISTS execution_id TEXT NOT NULL DEFAULT '';
