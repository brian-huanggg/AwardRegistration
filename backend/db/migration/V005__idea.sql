-- V005 (B3-01): idea, one row per team (PRD 6.6). There is no Idea without slides, so the slide metadata
-- lives on this row as NOT NULL columns (D22). Members are added by V006 (B5-01, idea_member).
--   category / org / direction  closed value sets (VARCHAR + CHECK, never ENUM)
--   name     1-75 code points, utf8mb4_bin: case-sensitive, unique per leader (D20), not globally
--   summary  1-300 code points
--   slides_object_key  slides/{ideaId}/{uuid}.{ext} (D50); the clean-up job asks "does any idea reference this key?"
--   slides_size        1..5 MiB (5,242,880 bytes, PRD 6.8)
--   version  optimistic-locking counter (PRD 9 #3); every successful UPDATE of the row adds 1
-- The text clean-up (NFC, whitespace) is done by shared/text before the INSERT; the CHECKs are the second line.

CREATE TABLE idea (
    id                UUID          NOT NULL DEFAULT UUID(),
    leader_id         UUID          NOT NULL,
    category          VARCHAR(24)   NOT NULL,
    org               VARCHAR(4)    NOT NULL,
    direction         VARCHAR(24)   NOT NULL,
    name              VARCHAR(75)   NOT NULL,
    summary           VARCHAR(300)  NOT NULL,
    slides_object_key VARCHAR(128)  NOT NULL,
    slides_ext        VARCHAR(4)    NOT NULL,
    slides_size       INT UNSIGNED  NOT NULL,
    slides_sha256     CHAR(64)      NOT NULL,
    slides_updated_at DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version           INT UNSIGNED  NOT NULL DEFAULT 0,
    created_at        DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    -- A join request addresses an Idea by (leader employee ID, name) (PRD 6.6); this is also the IDEA_DUPLICATE guard.
    UNIQUE KEY uq_idea_leader_name (leader_id, name),
    -- Deleting a User is not a supported operation while they lead an Idea.
    CONSTRAINT fk_idea_leader FOREIGN KEY (leader_id) REFERENCES app_user (id),
    CONSTRAINT ck_idea_category CHECK (category IN ('COOL_IDEA', 'OUTSTANDING_IMPACT')),
    CONSTRAINT ck_idea_org CHECK (org IN ('AAID', 'BSID', 'ICSD', 'TSID', 'PLED', 'DEID')),
    CONSTRAINT ck_idea_direction CHECK (direction IN ('GREEN_MANUFACTURING', 'RESPONSIBLE_SUPPLY_CHAIN',
        'INCLUSIVE_WORKPLACE', 'TALENT_DEVELOPMENT', 'CARING_FOR_DISADVANTAGED')),
    CONSTRAINT ck_idea_name CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 75
        AND name NOT REGEXP '[[:cntrl:]]' AND name NOT REGEXP '[/\\\\:*?"<>|]'),
    CONSTRAINT ck_idea_summary CHECK (CHAR_LENGTH(summary) BETWEEN 1 AND 300),
    CONSTRAINT ck_idea_slides_ext CHECK (slides_ext IN ('pdf', 'pptx')),
    CONSTRAINT ck_idea_slides_size CHECK (slides_size BETWEEN 1 AND 5242880),
    CONSTRAINT ck_idea_slides_sha256 CHECK (slides_sha256 REGEXP '^[0-9a-f]{64}$'),
    -- "My Ideas" and the Leader filter; the visibility query starts from here for Users.
    KEY ix_idea_leader (leader_id, created_at, id),
    -- Default list order (createdAt, id as final tie-breaker) and the name order.
    KEY ix_idea_created (created_at, id),
    KEY ix_idea_name (name, id),
    -- Orphan-file check: "no idea row has slides_object_key = ?"
    KEY ix_idea_slides_key (slides_object_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
