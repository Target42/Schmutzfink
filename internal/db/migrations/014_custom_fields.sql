CREATE TABLE custom_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants (id),
    key TEXT NOT NULL,
    label TEXT NOT NULL,
    field_type TEXT NOT NULL CHECK (field_type IN ('text', 'number', 'date', 'bool', 'select')),
    options TEXT[] NOT NULL DEFAULT '{}',
    required BOOLEAN NOT NULL DEFAULT false,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);

CREATE TABLE sighting_field_values (
    sighting_id UUID NOT NULL REFERENCES sightings (id) ON DELETE CASCADE,
    field_id UUID NOT NULL REFERENCES custom_fields (id) ON DELETE CASCADE,
    value_text TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (sighting_id, field_id)
);

CREATE INDEX sighting_field_values_field_idx ON sighting_field_values (field_id, value_text);
CREATE INDEX custom_fields_tenant_sort_idx ON custom_fields (tenant_id, sort_order, label);
