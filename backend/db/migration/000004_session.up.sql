-- V004 (B1-07): server-side sessions (PRD 6.5). Only SHA-256(session ID) is stored, never the ID itself,
-- so a database read does not yield usable cookies. The role is NOT stored: it is read from app_user on
-- every request. Times are database time in UTC (Architecture principle 4).
--   idle timeout     30 min since last_access_at (refreshed at most once per 60 s, OI-11)
--   absolute timeout  8 h since created_at
-- Deleting or disabling a user ends their sessions: ON DELETE CASCADE here, SessionRevoker for disable.

CREATE TABLE session (
    id_hash        BINARY(32)  NOT NULL,
    user_id        UUID        NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_access_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id_hash),
    KEY ix_session_user (user_id),
    -- Purge scans by age.
    KEY ix_session_created (created_at),
    KEY ix_session_last_access (last_access_at),
    CONSTRAINT fk_session_user FOREIGN KEY (user_id) REFERENCES app_user (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
