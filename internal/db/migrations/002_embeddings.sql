CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE records ADD COLUMN IF NOT EXISTS embedding vector(512);
ALTER TABLE records ADD COLUMN IF NOT EXISTS embedding_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS records_embedding_hnsw_idx
    ON records
    USING hnsw (embedding vector_cosine_ops);
