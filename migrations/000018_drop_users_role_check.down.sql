-- Revert: restore role CHECK constraint
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user'));
