CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY,
    code varchar(50) NOT NULL UNIQUE,
    name varchar(100) NOT NULL,
    description varchar(255),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    deleted_at timestamp
);

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    fullname varchar(255) NOT NULL,
    email varchar(255) NOT NULL UNIQUE,
    password_hash varchar(255) NOT NULL,
    role_id uuid NOT NULL REFERENCES roles(id),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    deleted_at timestamp
);

INSERT INTO roles (id, code, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'member', 'Member', 'Default app member role')
ON CONFLICT (code) DO NOTHING;
