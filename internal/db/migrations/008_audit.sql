CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id UUID REFERENCES tenants (id),
    user_id UUID REFERENCES users (id) ON DELETE SET NULL,
    username TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    detail JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_events_tenant_at_idx ON audit_events (tenant_id, at DESC);
CREATE INDEX audit_events_action_at_idx ON audit_events (action, at DESC);
