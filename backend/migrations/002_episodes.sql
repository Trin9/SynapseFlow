-- Sprint 7: Episode Schema
-- Creates the episodes and episode_artifacts tables that implement the
-- Episode product abstraction (action_verification / investigation_step).

CREATE TABLE IF NOT EXISTS episodes (
    id             TEXT        PRIMARY KEY,
    exec_id        TEXT        NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    episode_type   TEXT        NOT NULL,                         -- 'action_verification' | 'investigation_step'
    handles        JSONB       NOT NULL DEFAULT '{}',            -- {trace_id, order_id, alert_id, …}
    evidence       JSONB       NOT NULL DEFAULT '[]',            -- []EpisodeEvidence
    verdict        JSONB       NOT NULL DEFAULT '{}',            -- EpisodeVerdict (confidence, causal_chain, gaps)
    loop_guard     JSONB       NOT NULL DEFAULT '{}',            -- {max_iterations, attempted_actions, current_iteration}
    audit_trail    JSONB       NOT NULL DEFAULT '[]',            -- []EpisodeAuditEntry (human corrections)
    schema_version INTEGER     NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS episode_artifacts (
    id           TEXT        PRIMARY KEY,
    episode_id   TEXT        NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    evidence_id  TEXT        NOT NULL,                           -- links to EpisodeEvidence.ID
    content_type TEXT        NOT NULL,                           -- 'log_dump' | 'trace_export' | 'screenshot' | 'raw'
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    storage_uri  TEXT        NOT NULL,                           -- 'artifact://{exec_id}/{ev_id}'
    content      TEXT,                                           -- NULL when externalised; small payloads may be inlined
    created_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_episodes_exec_id         ON episodes(exec_id);
CREATE INDEX IF NOT EXISTS idx_episodes_type            ON episodes(episode_type);
CREATE INDEX IF NOT EXISTS idx_episode_artifacts_ep     ON episode_artifacts(episode_id);
CREATE INDEX IF NOT EXISTS idx_episode_artifacts_ev     ON episode_artifacts(evidence_id);
