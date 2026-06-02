-- Make ai_provider_file_id nullable so we can create the job before uploading.
ALTER TABLE ingestion_job ALTER COLUMN ai_provider_file_id DROP NOT NULL;
ALTER TABLE ingestion_job ALTER COLUMN ai_provider_file_id SET DEFAULT NULL;
