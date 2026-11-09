-- V010 (B5-01): idea_member, one row per join request or approved Member (PRD 6.7, D19).
--   status       PENDING | ACCEPTED. Reject, withdraw, remove and leave DELETE the row; history lives in the Audit Log.
--   accepted_at  set exactly when status = 'ACCEPTED' (CHECK).
--   PK (idea_id, user_id)  one row per User per Idea: this is the ALREADY_MEMBER / JOIN_REQUEST_EXISTS guard.
--   idea_id      ON DELETE CASCADE: deleting an Idea removes its members and requests (Sequence.md delete flow).
--   user_id      no cascade: deleting a User is not a supported operation while they hold memberships.
-- "The Leader is not a Member" and "approved Members <= 3" span tables, so the service enforces them
-- (capacity under the Idea row lock, B5-04); no CHECK can.

CREATE TABLE idea_member (
    idea_id     UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    status      VARCHAR(8)  NOT NULL DEFAULT 'PENDING',
    accepted_at DATETIME(6) NULL,
    created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (idea_id, user_id),
    CONSTRAINT fk_idea_member_idea FOREIGN KEY (idea_id) REFERENCES idea (id) ON DELETE CASCADE,
    CONSTRAINT fk_idea_member_user FOREIGN KEY (user_id) REFERENCES app_user (id),
    CONSTRAINT ck_idea_member_status CHECK (status IN ('PENDING', 'ACCEPTED')),
    CONSTRAINT ck_idea_member_accepted CHECK ((status = 'ACCEPTED') = (accepted_at IS NOT NULL)),
    -- Visibility ("is this User an approved Member?"), "My Ideas" and "my pending requests".
    KEY ix_idea_member_user (user_id, status)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;
