CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_type_enum') THEN
        CREATE TYPE transaction_type_enum AS ENUM ('income', 'expense');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'split_type_enum') THEN
        CREATE TYPE split_type_enum AS ENUM ('ratio', 'fixed');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS expense_categories (
    id uuid PRIMARY KEY,
    code varchar(100) NOT NULL UNIQUE,
    name varchar(150) NOT NULL,
    description varchar(255),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz
);

CREATE TABLE IF NOT EXISTS personal_transactions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    user_fullname varchar(255) NOT NULL,
    user_email varchar(255) NOT NULL,
    category_id uuid REFERENCES expense_categories(id),
    category_code varchar(100),
    category_name varchar(150),
    type transaction_type_enum NOT NULL,
    title varchar(255) NOT NULL,
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    currency char(3) NOT NULL DEFAULT 'VND',
    transaction_date timestamptz NOT NULL,
    note varchar(500),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz
);

CREATE TABLE IF NOT EXISTS group_transactions (
    id uuid PRIMARY KEY,
    group_id uuid NOT NULL,
    group_name varchar(255) NOT NULL,
    category_id uuid REFERENCES expense_categories(id),
    category_code varchar(100),
    category_name varchar(150),
    type transaction_type_enum NOT NULL,
    title varchar(255) NOT NULL,
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    currency char(3) NOT NULL DEFAULT 'VND',
    transaction_date timestamptz NOT NULL,
    note varchar(500),
    created_by uuid NOT NULL,
    created_by_fullname varchar(255) NOT NULL,
    created_by_email varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz
);

CREATE TABLE IF NOT EXISTS group_transaction_payments (
    id uuid PRIMARY KEY,
    transaction_id uuid NOT NULL REFERENCES group_transactions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    user_fullname varchar(255) NOT NULL,
    user_email varchar(255) NOT NULL,
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    note varchar(255),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (transaction_id, user_id)
);

CREATE TABLE IF NOT EXISTS group_transaction_splits (
    id uuid PRIMARY KEY,
    transaction_id uuid NOT NULL REFERENCES group_transactions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    user_fullname varchar(255) NOT NULL,
    user_email varchar(255) NOT NULL,
    split_type split_type_enum NOT NULL,
    split_value numeric(18,6) NOT NULL CHECK (split_value > 0),
    share_amount numeric(18,2) NOT NULL CHECK (share_amount > 0),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (transaction_id, user_id)
);

CREATE TABLE IF NOT EXISTS settlements (
    id uuid PRIMARY KEY,
    group_id uuid NOT NULL,
    group_name varchar(255) NOT NULL,
    from_user_id uuid NOT NULL,
    from_user_fullname varchar(255) NOT NULL,
    from_user_email varchar(255) NOT NULL,
    to_user_id uuid NOT NULL,
    to_user_fullname varchar(255) NOT NULL,
    to_user_email varchar(255) NOT NULL,
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    currency char(3) NOT NULL DEFAULT 'VND',
    settled_at timestamptz NOT NULL,
    note varchar(255),
    created_by uuid NOT NULL,
    created_by_fullname varchar(255) NOT NULL,
    created_by_email varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    deleted_at timestamptz,
    CHECK (from_user_id <> to_user_id)
);

CREATE INDEX IF NOT EXISTS idx_personal_transactions_user_date ON personal_transactions(user_id, transaction_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_personal_transactions_category ON personal_transactions(category_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_group_transactions_group_date ON group_transactions(group_id, transaction_date DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_group_transactions_category ON group_transactions(category_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_group_payments_user ON group_transaction_payments(user_id);
CREATE INDEX IF NOT EXISTS idx_group_splits_user ON group_transaction_splits(user_id);
CREATE INDEX IF NOT EXISTS idx_settlements_group_date ON settlements(group_id, settled_at DESC) WHERE deleted_at IS NULL;
