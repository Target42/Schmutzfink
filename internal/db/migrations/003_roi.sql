ALTER TABLE records
    ADD COLUMN IF NOT EXISTS roi_x DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS roi_y DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS roi_w DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS roi_h DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS embedding_gen INTEGER NOT NULL DEFAULT 0;

ALTER TABLE records ADD CONSTRAINT records_roi_complete CHECK (
    (roi_x IS NULL AND roi_y IS NULL AND roi_w IS NULL AND roi_h IS NULL)
    OR (
        roi_x IS NOT NULL AND roi_y IS NOT NULL AND roi_w IS NOT NULL AND roi_h IS NOT NULL
        AND roi_x >= 0 AND roi_y >= 0
        AND roi_w >= 0.01 AND roi_h >= 0.01
        AND roi_x + roi_w <= 1.000001
        AND roi_y + roi_h <= 1.000001
    )
);
