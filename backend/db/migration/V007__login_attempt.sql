-- V007 (B1b-05): per-IP login failure events (PRD 6.5). One row per failed login attempt; the IP limit counts
-- the rows of one address inside a sliding 15-minute window (30 failures -> 429). Throttled attempts are not
-- recorded. All times are database time. Rows older than the window are dead weight; the purge job (B1b-07)
-- deletes them by attempted_at.

CREATE TABLE login_attempt (
    id           BIGINT      NOT NULL AUTO_INCREMENT,
    ip           INET6       NOT NULL,
    attempted_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    -- Window count and "the 30th newest" lookup per address.
    KEY ix_login_attempt_ip_time (ip, attempted_at),
    -- Purge scans by age.
    KEY ix_login_attempt_time (attempted_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
