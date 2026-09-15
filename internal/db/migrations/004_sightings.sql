CREATE TABLE sightings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    record_id UUID NOT NULL REFERENCES records (id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants (id),
    note TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',
    roi_x DOUBLE PRECISION,
    roi_y DOUBLE PRECISION,
    roi_w DOUBLE PRECISION,
    roi_h DOUBLE PRECISION,
    embedding vector(512),
    embedding_error TEXT NOT NULL DEFAULT '',
    embedding_status TEXT NOT NULL DEFAULT 'pending',
    embedding_gen INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sightings_roi_complete CHECK (
        (roi_x IS NULL AND roi_y IS NULL AND roi_w IS NULL AND roi_h IS NULL)
        OR (
            roi_x IS NOT NULL AND roi_y IS NOT NULL AND roi_w IS NOT NULL AND roi_h IS NOT NULL
            AND roi_x >= 0 AND roi_y >= 0
            AND roi_w >= 0.01 AND roi_h >= 0.01
            AND roi_x + roi_w <= 1.000001
            AND roi_y + roi_h <= 1.000001
        )
    )
);

INSERT INTO sightings (
    record_id, tenant_id, note, tags,
    roi_x, roi_y, roi_w, roi_h,
    embedding, embedding_error, embedding_status, embedding_gen,
    created_at, updated_at
)
SELECT
    id, tenant_id, note, tags,
    roi_x, roi_y, roi_w, roi_h,
    embedding, embedding_error, embedding_status, embedding_gen,
    created_at, updated_at
FROM records;

CREATE INDEX sightings_record_idx ON sightings (record_id);
CREATE INDEX sightings_tenant_created_idx ON sightings (tenant_id, created_at);
CREATE INDEX sightings_pending_idx ON sightings (created_at) WHERE embedding_status = 'pending';
CREATE INDEX sightings_tags_idx ON sightings USING gin (tags);
CREATE INDEX sightings_embedding_hnsw_idx
    ON sightings
    USING hnsw (embedding vector_cosine_ops);

DROP INDEX IF EXISTS records_embedding_hnsw_idx;
DROP INDEX IF EXISTS records_tags_idx;

ALTER TABLE records
    DROP CONSTRAINT IF EXISTS records_roi_complete,
    DROP COLUMN IF EXISTS note,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS embedding,
    DROP COLUMN IF EXISTS embedding_error,
    DROP COLUMN IF EXISTS embedding_status,
    DROP COLUMN IF EXISTS embedding_gen,
    DROP COLUMN IF EXISTS roi_x,
    DROP COLUMN IF EXISTS roi_y,
    DROP COLUMN IF EXISTS roi_w,
    DROP COLUMN IF EXISTS roi_h;
