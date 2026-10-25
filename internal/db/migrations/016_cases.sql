CREATE TABLE cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants (id),
    title TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('civil', 'criminal')),
    created_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE records
    ADD COLUMN case_id UUID REFERENCES cases (id) ON DELETE SET NULL,
    ADD COLUMN location_redacted_at TIMESTAMPTZ;

CREATE INDEX cases_tenant_updated_idx ON cases (tenant_id, updated_at DESC);
CREATE INDEX records_case_idx ON records (case_id) WHERE case_id IS NOT NULL;
CREATE INDEX records_location_due_idx
    ON records (tenant_id, case_id)
    WHERE location_redacted_at IS NULL AND case_id IS NOT NULL;
