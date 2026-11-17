CREATE TABLE motifs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants (id),
    title TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE sightings
    ADD COLUMN motif_id UUID REFERENCES motifs (id) ON DELETE SET NULL;

CREATE INDEX motifs_tenant_updated_idx ON motifs (tenant_id, updated_at DESC);
CREATE INDEX sightings_motif_idx ON sightings (motif_id) WHERE motif_id IS NOT NULL;
