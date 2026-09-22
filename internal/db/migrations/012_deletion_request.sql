ALTER TABLE records
    ADD COLUMN deletion_requested_at TIMESTAMPTZ,
    ADD COLUMN deletion_requested_by UUID REFERENCES users (id),
    ADD COLUMN deletion_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX records_deletion_pending_idx
    ON records (tenant_id, deletion_requested_at)
    WHERE deletion_requested_at IS NOT NULL;
