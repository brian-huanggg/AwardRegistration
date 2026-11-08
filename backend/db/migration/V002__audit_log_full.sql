-- V002 (B1-03): the remaining audit_log columns (PRD 6.11 "Fields"). Expand-only: V001 created
-- the skeleton (id, occurred_at); the table is still empty, so NOT NULL columns are safe.
-- Append-only: application code only INSERTs (test/arch scans for anything else).
-- "before" / "after" are reserved words in MariaDB, hence before_json / after_json.
-- No foreign keys: a row must outlive the user or Idea it describes.

ALTER TABLE audit_log
    ADD COLUMN request_id       UUID              NULL     COMMENT 'X-Request-Id of the request' AFTER occurred_at,
    ADD COLUMN actor_emp_no     VARCHAR(64)       NULL     COMMENT 'NULL for ANONYMOUS and SYSTEM' AFTER request_id,
    ADD COLUMN actor_role       VARCHAR(16)       NOT NULL AFTER actor_emp_no,
    ADD COLUMN action           VARCHAR(40)       NOT NULL AFTER actor_role,
    ADD COLUMN outcome          VARCHAR(16)       NOT NULL AFTER action,
    ADD COLUMN target_type      VARCHAR(16)       NULL     AFTER outcome,
    ADD COLUMN target_id        UUID              NULL     AFTER target_type,
    ADD COLUMN method           VARCHAR(8)        NULL     AFTER target_id,
    ADD COLUMN route            VARCHAR(200)      NULL     COMMENT 'route template, never the raw path' AFTER method,
    ADD COLUMN status           SMALLINT UNSIGNED NULL     AFTER route,
    ADD COLUMN error_code       VARCHAR(64)       NULL     AFTER status,
    ADD COLUMN ip               INET6             NULL     AFTER error_code,
    ADD COLUMN user_agent       VARCHAR(256)      NULL     AFTER ip,
    ADD COLUMN submitted_emp_no VARCHAR(64)       NULL     COMMENT 'sanitized employee ID typed into a failed login or registration' AFTER user_agent,
    ADD COLUMN before_json      JSON              NULL     AFTER submitted_emp_no,
    ADD COLUMN after_json       JSON              NULL     AFTER before_json,
    -- Reserved for the optional hash chain (PRD 9 #5); nothing writes them yet.
    ADD COLUMN prev_hash        BINARY(32)        NULL,
    ADD COLUMN hash             BINARY(32)        NULL,
    ADD CONSTRAINT ck_audit_log_actor_role  CHECK (actor_role IN ('USER', 'ADMIN', 'ANONYMOUS', 'SYSTEM')),
    ADD CONSTRAINT ck_audit_log_outcome     CHECK (outcome IN ('SUCCESS', 'FAILED', 'DENIED', 'THROTTLED')),
    ADD CONSTRAINT ck_audit_log_target_type CHECK (target_type IS NULL OR target_type IN ('USER', 'IDEA', 'DEADLINE')),
    ADD KEY ix_audit_log_actor (actor_emp_no, occurred_at, id),
    ADD KEY ix_audit_log_request (request_id),
    ADD KEY ix_audit_log_target (target_id, occurred_at, id);
