ALTER TABLE cases
    ADD COLUMN closed_at TIMESTAMPTZ,
    ADD COLUMN closed_by UUID REFERENCES users (id);

CREATE INDEX cases_closed_due_idx
    ON cases (tenant_id, closed_at)
    WHERE closed_at IS NOT NULL;
