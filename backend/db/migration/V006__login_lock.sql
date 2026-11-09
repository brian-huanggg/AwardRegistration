-- V006 (B1b-04): per-key login lock (PRD 6.5, OI-12). One row per submitted employee-ID string (control
-- characters stripped, cut to 32 chars), also for accounts that do not exist. All times are database time.
--   fail_count   consecutive failures, each within 15 min of the previous one
--   locked_until set by the 5th failure to NOW + 15 min (fixed; later failures never extend it)
-- The IP sliding window (login_attempt) arrives with B1b-05.

CREATE TABLE login_lock (
    lock_key     VARCHAR(32) NOT NULL,
    fail_count   INT         NOT NULL,
    last_fail_at DATETIME(6) NOT NULL,
    locked_until DATETIME(6) NULL,
    PRIMARY KEY (lock_key),
    -- Purge scans by age.
    KEY ix_login_lock_last_fail (last_fail_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
