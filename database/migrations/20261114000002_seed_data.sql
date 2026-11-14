-- +goose Up
-- Initial Seed Data for Multi-Tenant User Management System

-- 1. Standard Permissions (resource:action)
INSERT INTO permissions (id, resource, action, description) VALUES
    ('00000000-0000-0000-0000-000000000001', 'user', 'create', 'Create users in tenant'),
    ('00000000-0000-0000-0000-000000000002', 'user', 'read', 'List and view users in tenant'),
    ('00000000-0000-0000-0000-000000000003', 'user', 'update', 'Update user details in tenant'),
    ('00000000-0000-0000-0000-000000000004', 'user', 'delete', 'Soft-delete users in tenant'),
    ('00000000-0000-0000-0000-000000000005', 'role', 'create', 'Create tenant-specific roles'),
    ('00000000-0000-0000-0000-000000000006', 'role', 'read', 'View tenant roles and permissions'),
    ('00000000-0000-0000-0000-000000000007', 'role', 'update', 'Update tenant roles and permissions'),
    ('00000000-0000-0000-0000-000000000008', 'role', 'delete', 'Delete tenant roles'),
    ('00000000-0000-0000-0000-000000000009', 'role', 'assign', 'Assign roles to users in tenant'),
    ('00000000-0000-0000-0000-000000000010', 'tenant', 'read', 'View tenant profile and details'),
    ('00000000-0000-0000-0000-000000000011', 'tenant', 'update', 'Update tenant profile'),
    ('00000000-0000-0000-0000-000000000012', 'audit', 'read', 'View tenant audit logs')
ON CONFLICT (resource, action) DO NOTHING;

-- 2. Initial Tenants
INSERT INTO tenants (id, name, code, status) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Acme Corporation', 'acme-corp', 'active'),
    ('a0000000-0000-0000-0000-000000000002', 'Globex Corporation', 'globex', 'active'),
    ('a0000000-0000-0000-0000-000000000003', 'Initech Suspended', 'initech', 'suspended')
ON CONFLICT (code) DO NOTHING;

-- 3. Initial Users (All default passwords: "Password123!")
-- Bcrypt hash generated with cost 10 for Password123!
INSERT INTO users (id, email, password_hash, first_name, last_name, is_platform_admin) VALUES
    ('10000000-0000-0000-0000-000000000001', 'admin@platform.com', '$2a$10$iZg7t13h7b9xQxW5Zg/7g.pY5P1m9UoKjJ0X9rU5cZ7F5vU5g7K7e', 'Platform', 'Admin', TRUE),
    ('10000000-0000-0000-0000-000000000002', 'alice@multitenant.com', '$2a$10$iZg7t13h7b9xQxW5Zg/7g.pY5P1m9UoKjJ0X9rU5cZ7F5vU5g7K7e', 'Alice', 'Multi', FALSE),
    ('10000000-0000-0000-0000-000000000003', 'bob@acme.com', '$2a$10$iZg7t13h7b9xQxW5Zg/7g.pY5P1m9UoKjJ0X9rU5cZ7F5vU5g7K7e', 'Bob', 'Acme', FALSE),
    ('10000000-0000-0000-0000-000000000004', 'charlie@globex.com', '$2a$10$iZg7t13h7b9xQxW5Zg/7g.pY5P1m9UoKjJ0X9rU5cZ7F5vU5g7K7e', 'Charlie', 'Globex', FALSE)
ON CONFLICT (email) DO NOTHING;

-- 4. Tenant Users (Alice belongs to both Acme and Globex!)
INSERT INTO tenant_users (id, tenant_id, user_id, status) VALUES
    ('20000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002', 'active'),
    ('20000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002', 'active'),
    ('20000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000003', 'active'),
    ('20000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000004', 'active')
ON CONFLICT (tenant_id, user_id) DO NOTHING;

-- 5. Tenant-specific Roles
-- Acme roles: Admin, Member
INSERT INTO roles (id, tenant_id, name, description, is_system) VALUES
    ('30000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'Admin', 'Full administrative access in Acme', TRUE),
    ('30000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'Member', 'Standard member access in Acme', FALSE),
-- Globex roles: Admin, Auditor
    ('30000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000002', 'Admin', 'Full administrative access in Globex', TRUE),
    ('30000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000002', 'Auditor', 'Audit and read access in Globex', FALSE)
ON CONFLICT (tenant_id, name) DO NOTHING;

-- 6. Role Permissions mapping
-- Acme Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT '30000000-0000-0000-0000-000000000001', id FROM permissions
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Acme Member gets user:read, tenant:read
INSERT INTO role_permissions (role_id, permission_id)
SELECT '30000000-0000-0000-0000-000000000002', id FROM permissions 
WHERE (resource = 'user' AND action = 'read') OR (resource = 'tenant' AND action = 'read')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Globex Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT '30000000-0000-0000-0000-000000000003', id FROM permissions
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Globex Auditor gets user:read, role:read, tenant:read, audit:read
INSERT INTO role_permissions (role_id, permission_id)
SELECT '30000000-0000-0000-0000-000000000004', id FROM permissions 
WHERE action = 'read'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- 7. User Roles (Alice is Admin in Acme, but Auditor in Globex! Bob is Admin in Acme, Charlie is Auditor in Globex)
INSERT INTO user_roles (tenant_id, user_id, role_id) VALUES
    ('a0000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001'),
    ('a0000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000004'),
    ('a0000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000001'),
    ('a0000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000004')
ON CONFLICT (tenant_id, user_id, role_id) DO NOTHING;

-- +goose Down
DELETE FROM user_roles WHERE tenant_id IN ('a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000002');
DELETE FROM role_permissions WHERE role_id IN ('30000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000004');
DELETE FROM roles WHERE tenant_id IN ('a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000002');
DELETE FROM tenant_users WHERE tenant_id IN ('a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000002');
DELETE FROM users WHERE id IN ('10000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000004');
DELETE FROM tenants WHERE id IN ('a0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000003');
DELETE FROM permissions WHERE id LIKE '00000000-0000-0000-0000-0000000000%';
