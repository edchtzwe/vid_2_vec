DROP TRIGGER IF EXISTS update_ingestion_job_modtime ON ingestion_job;
DROP FUNCTION IF EXISTS update_modified_column;
DROP TABLE IF EXISTS ingestion_job;