CREATE TABLE IF NOT EXISTS menus (
    id uuid PRIMARY KEY,
    code varchar(100) NOT NULL UNIQUE,
    name varchar(150) NOT NULL,
    route varchar(255) UNIQUE,
    icon varchar(100),
    parent_id uuid REFERENCES menus(id),
    sort_order int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp NOT NULL DEFAULT NOW(),
    updated_at timestamp NOT NULL DEFAULT NOW(),
    deleted_at timestamp
);

CREATE INDEX IF NOT EXISTS idx_menus_parent_id ON menus(parent_id);
CREATE INDEX IF NOT EXISTS idx_menus_sort_order ON menus(sort_order, created_at);

INSERT INTO menus (id, code, name, route, icon, parent_id, sort_order, is_active)
VALUES
    ('10000000-0000-0000-0000-000000000001', 'dashboard', 'Dashboard', '/dashboard', 'layout-dashboard', NULL, 10, true),
    ('10000000-0000-0000-0000-000000000002', 'profile', 'Profile', '/profile', 'user-circle', NULL, 20, true),
    ('10000000-0000-0000-0000-000000000003', 'transactions', 'Transactions', '/transactions', 'wallet', NULL, 30, true),
    ('10000000-0000-0000-0000-000000000004', 'groups', 'Groups', '/groups', 'users', NULL, 40, true),
    ('10000000-0000-0000-0000-000000000005', 'settings', 'Settings', '/settings', 'settings', NULL, 50, true)
ON CONFLICT (code) DO NOTHING;
