ALTER TABLE ingestion_job
    ADD CONSTRAINT fk_ingestion_job_source_video
    FOREIGN KEY (source_video_id) REFERENCES source_video(id);
