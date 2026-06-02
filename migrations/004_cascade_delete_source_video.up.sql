ALTER TABLE ingestion_job
    DROP CONSTRAINT IF EXISTS fk_ingestion_job_source_video;

ALTER TABLE ingestion_job
    ADD CONSTRAINT fk_ingestion_job_source_video
    FOREIGN KEY (source_video_id) REFERENCES source_video(id)
    ON DELETE CASCADE;
