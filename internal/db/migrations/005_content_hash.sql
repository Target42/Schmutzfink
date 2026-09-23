ALTER TABLE records
    ADD COLUMN IF NOT EXISTS content_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS records_tenant_content_hash_uidx
    ON records (tenant_id, content_hash)
    WHERE content_hash <> '';
