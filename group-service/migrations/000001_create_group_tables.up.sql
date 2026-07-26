CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'member_role_enum') THEN
        CREATE TYPE member_role_enum AS ENUM ('owner', 'member');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invitation_status_enum') THEN
        CREATE TYPE invitation_status_enum AS ENUM ('pending', 'accepted', 'rejected', 'expired');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS groups (
    id uuid PRIMARY KEY,
    name varchar(255) NOT NULL,
    description varchar(255),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    deleted_at timestamp
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id uuid NOT NULL REFERENCES groups(id),
    user_id uuid NOT NULL,
    role member_role_enum NOT NULL DEFAULT 'member',
    joined_at timestamp NOT NULL DEFAULT NOW(),
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS group_invitations (
    id uuid PRIMARY KEY,
    group_id uuid NOT NULL REFERENCES groups(id),
    invited_user_id uuid NOT NULL,
    invited_by uuid NOT NULL,
    role member_role_enum NOT NULL DEFAULT 'member',
    status invitation_status_enum NOT NULL DEFAULT 'pending',
    token varchar(255) NOT NULL UNIQUE,
    expired_at timestamp,
    responded_at timestamp,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    deleted_at timestamp
);

CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_group_invites_group_id ON group_invitations(group_id);
CREATE INDEX IF NOT EXISTS idx_group_invites_invited_user_id ON group_invitations(invited_user_id);

