-- V003 (B1-05): app_user, the one table for Users and Admins (PRD 4, 6.3).
-- Field rules are enforced here as well as in Go, so no code path can store a value the API would reject.
--   emp_no     exactly 6 ASCII digits (string; keeps leading zeros)
--   name       1-50 code points, no control characters, none of  / \ : * ? " < > |  (PRD 6.2 #5, #8);
--              the NFC / whitespace clean-up itself is done by shared/text before the INSERT
--   email      printable ASCII only, no whitespace, one '@' with non-empty sides, <= 254, already lower-cased (OI-14);
--              unique byte-exact (utf8mb4_bin). NULL only for Admins, who have no email (PRD 4).
--   password_hash  bcrypt, cost 10 only (PRD 6.4, OI-13)
-- Enumerations are VARCHAR + CHECK, never ENUM. Times are database time in UTC.

CREATE TABLE app_user (
    id            UUID         NOT NULL DEFAULT UUID(),
    emp_no        CHAR(6)      NOT NULL,
    name          VARCHAR(50)  NOT NULL,
    email         VARCHAR(254) NULL,
    password_hash CHAR(60)     NOT NULL,
    role          VARCHAR(8)   NOT NULL,
    -- Every authenticated request checks this (PRD 4); an Admin removed from Vault is disabled, not deleted.
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_app_user_emp_no (emp_no),
    -- Several NULLs are allowed (Admins); non-NULL values are unique.
    UNIQUE KEY uq_app_user_email (email),
    CONSTRAINT ck_app_user_emp_no CHECK (CHAR_LENGTH(emp_no) = 6 AND emp_no NOT REGEXP '[^0-9]'),
    CONSTRAINT ck_app_user_name_length CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 50),
    CONSTRAINT ck_app_user_name_chars CHECK (name NOT REGEXP '[[:cntrl:]]' AND name NOT REGEXP '[/\\\\:*?"<>|]'),
    CONSTRAINT ck_app_user_email CHECK (email IS NULL OR (
        email NOT REGEXP '[^!-~]'
        AND email = LOWER(email)
        AND email REGEXP '^[^@]+@[^@]+$')),
    CONSTRAINT ck_app_user_hash CHECK (password_hash REGEXP '^[$]2[aby][$]10[$][./A-Za-z0-9]{53}$'),
    CONSTRAINT ck_app_user_role CHECK (role IN ('USER', 'ADMIN')),
    CONSTRAINT ck_app_user_email_by_role CHECK ((role = 'USER' AND email IS NOT NULL) OR (role = 'ADMIN' AND email IS NULL))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
