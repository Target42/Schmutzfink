UPDATE users
SET username = btrim(username)
WHERE username IS DISTINCT FROM btrim(username);

CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_username_ci
    ON users (tenant_id, lower(username));
