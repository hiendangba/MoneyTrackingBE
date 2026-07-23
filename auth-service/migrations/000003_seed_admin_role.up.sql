INSERT INTO roles (id, code, name, description)
VALUES ('00000000-0000-0000-0000-000000000002', 'admin', 'Admin', 'Application administrator role')
ON CONFLICT (code) DO NOTHING;
