ALTER TABLE users
    ADD COLUMN IF NOT EXISTS can_delete BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user', 'searcher'));

UPDATE users
SET can_delete = true
WHERE role = 'admin';
