CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ==========================================
-- 1. Images Table
-- Stores metadata of the original uploaded image
-- ==========================================
CREATE TABLE IF NOT EXISTS images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    original_path TEXT NOT NULL,
    original_format VARCHAR(10) NOT NULL, -- e.g., 'jpeg', 'png', 'webp'
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ==========================================
-- 2. Jobs Table
-- Tracks background processing state, retries, and failure reasons
-- ==========================================
CREATE TABLE IF NOT EXISTS jobs (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        image_id UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
        status VARCHAR(20) NOT NULL DEFAULT 'pending',
        attempts INTEGER NOT NULL DEFAULT 0,
        max_attempts INTEGER NOT NULL DEFAULT 3,
        error_message TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Ensure valid status transitions
CONSTRAINT chk_job_status CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
    );

-- Index to quickly look up jobs by status and image
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);

CREATE INDEX IF NOT EXISTS idx_jobs_image_id ON jobs (image_id);

-- ==========================================
-- 3. Image Variants Table
-- Stores metadata of the 6 generated files per image
-- ==========================================
CREATE TABLE IF NOT EXISTS image_variants (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        image_id UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
        size_type VARCHAR(20) NOT NULL, -- 'thumbnail' or 'catalog'
        format VARCHAR(10) NOT NULL,    -- 'webp', 'jpeg', or 'png'
        path TEXT NOT NULL,
        width INTEGER NOT NULL,
        height INTEGER NOT NULL,
        file_size_bytes BIGINT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

-- Constraints for size types and formats
CONSTRAINT chk_variant_size_type CHECK (
    size_type IN ('thumbnail', 'catalog')
),
CONSTRAINT chk_variant_format CHECK (
    format IN ('webp', 'jpeg', 'png')
),

-- Idempotency constraint: an image cannot have duplicate variants of the same size and format
CONSTRAINT uq_image_variant UNIQUE (image_id, size_type, format)
    );

-- Index for querying variants by image
CREATE INDEX IF NOT EXISTS idx_image_variants_image_id ON image_variants (image_id);