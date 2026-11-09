-- V009 (B1b-06): "a throttled key is audited once per window" (PRD 6.11). One row per throttled subject records
-- until when its THROTTLED audit row has been written; a throttled attempt writes a new audit row only after it
-- wins the claim (expired row updated, or row inserted), so replicas agree. All times are database time.
--   scope  'key' (login_lock key) or 'ip' (source address)
-- Rows past audited_until are dead weight; the purge job (B1b-07) deletes them.

CREATE TABLE login_throttle_audit (
    scope         VARCHAR(8)  NOT NULL,
    subject       VARCHAR(45) NOT NULL,
    audited_until DATETIME(6) NOT NULL,
    PRIMARY KEY (scope, subject),
    KEY ix_login_throttle_audit_until (audited_until)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
