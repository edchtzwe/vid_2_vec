-- Restore NOT NULL constraint (backfill first to avoid failure on existing rows).
UPDATE ingestion_job SET ai_provider_file_id = '' WHERE ai_provider_file_id IS NULL;
ALTER TABLE ingestion_job ALTER COLUMN ai_provider_file_id SET NOT NULL;
ALTER TABLE ingestion_job ALTER COLUMN ai_provider_file_id SET DEFAULT '';
