-- Revert: restore NOT NULL constraints (only safe if no NULLs exist in data)
ALTER TABLE users
  ALTER COLUMN name SET NOT NULL,
  ALTER COLUMN tenant_id SET NOT NULL;
