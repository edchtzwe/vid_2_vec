-- 1. Create Table (No schema prefix)
-- Note: gen_random_uuid() is available in PostgreSQL 13+. 
-- For older versions, you may need: CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; and use uuid_generate_v4()
CREATE TABLE IF NOT EXISTS ingestion_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), 
    source_video_id UUID NOT NULL,
    ai_provider_file_id TEXT NOT NULL,
    ai_provider_name TEXT NOT NULL,
    ai_model_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PROVIDER_PROCESSING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Create Index
CREATE INDEX IF NOT EXISTS idx_ingestion_job_status 
    ON ingestion_job(status);

-- 3. Create Function
-- Kept generic to be reusable across other tables if needed
CREATE OR REPLACE FUNCTION update_modified_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 4. Create Trigger
DROP TRIGGER IF EXISTS update_ingestion_job_modtime ON ingestion_job;
CREATE TRIGGER update_ingestion_job_modtime
    BEFORE UPDATE ON ingestion_job
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();
