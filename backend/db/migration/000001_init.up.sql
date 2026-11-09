-- V001 (B0-01): first schema slice.
--   contest_setting  single-row table holding the submission deadline (PRD 5, 6.9)
--   audit_log        skeleton only; B1-03 adds the remaining columns (expand-only)
-- Times are database time in UTC; DATETIME has no zone, the DSN pins loc=UTC.

CREATE TABLE contest_setting (
    id         TINYINT UNSIGNED NOT NULL,
    -- NULL = no deadline ever set (state NOT_OPEN). Second precision (PRD 6.9).
    deadline   DATETIME(0)      NULL,
    updated_at DATETIME(6)      NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT ck_contest_setting_single_row CHECK (id = 1)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;

-- The one and only row; application code only ever UPDATEs it.
INSERT INTO contest_setting (id, deadline) VALUES (1, NULL);

CREATE TABLE audit_log (
    -- Sequence for the optional hash chain (PRD 9 #5); Admin-visible only.
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    -- Database time is the only clock (PRD 5).
    occurred_at DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY ix_audit_log_occurred_at_id (occurred_at, id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
