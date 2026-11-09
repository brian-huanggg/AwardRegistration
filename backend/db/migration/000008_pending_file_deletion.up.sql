-- V008 (B3-06): pending_file_deletion, MinIO object keys that should be deleted (PRD 6.8, Sequence.md 2, D25, D36).
-- Written in v1; the clean-up job that drains it is the next phase (PRD 9 #7). All times are database time.
--   object_key       slides/{ideaId}/{uuid}.{ext}; no FK to idea: the row must outlive the idea it came from
--   reason           REPLACED       old key, inserted in the transaction that switched the pointer
--                    DELETED        key of a deleted Idea, inserted in the delete transaction
--                    UPLOAD_ABORTED new key whose transaction rolled back (delete failed) or ended unknown;
--                                   the job deletes it only after next_attempt_at AND if no idea row references it
--   next_attempt_at  earliest time the job may act: NOW for REPLACED / DELETED / known rollback,
--                    NOW + grace period for an unknown outcome (OI-19, proposed 10 min)
--   attempts         failed job attempts so far (the job backs off with next_attempt_at)
-- A key may appear more than once (UPLOAD_ABORTED after a lost COMMIT ack, later REPLACED), so it is not unique.
-- A row is removed when the object was deleted, or when the job finds an UPLOAD_ABORTED key still referenced.

CREATE TABLE pending_file_deletion (
    id              BIGINT        NOT NULL AUTO_INCREMENT,
    object_key      VARCHAR(128)  NOT NULL,
    reason          VARCHAR(16)   NOT NULL,
    created_at      DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    next_attempt_at DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    attempts        INT UNSIGNED  NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    CONSTRAINT ck_pending_file_deletion_reason CHECK (reason IN ('REPLACED', 'DELETED', 'UPLOAD_ABORTED')),
    CONSTRAINT ck_pending_file_deletion_key CHECK (CHAR_LENGTH(object_key) BETWEEN 1 AND 128),
    -- The job claims due rows: WHERE next_attempt_at <= NOW(6) ORDER BY next_attempt_at, id FOR UPDATE SKIP LOCKED.
    KEY ix_pending_file_deletion_due (next_attempt_at, id),
    KEY ix_pending_file_deletion_key (object_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
