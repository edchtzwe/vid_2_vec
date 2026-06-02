package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

type Uploader struct {
	repo        *repository.Repository
	gemini      *gemini.Client
	asynqClient *asynq.Client
	sourceDir   string
	logger      *log.Logger
}

func NewUploader(repo *repository.Repository, geminiClient *gemini.Client, asynqClient *asynq.Client, sourceDir string, logger *log.Logger) *Uploader {
	return &Uploader{
		repo:        repo,
		gemini:      geminiClient,
		asynqClient: asynqClient,
		sourceDir:   sourceDir,
		logger:      logger,
	}
}

func (u *Uploader) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.JobPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	jobID := p.JobID

	job, err := u.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", jobID, err)
	}

	fullPath := filepath.Join(u.sourceDir, job.SourceVideoID+".mp4")
	u.logger.Printf("starting upload for job %s (file: %s)", jobID, fullPath)

	fileInfo, err := u.gemini.UploadFile(ctx, fullPath)
	if err != nil {
		u.logger.Printf("upload failed for job %s: %v", jobID, err)
		if markErr := u.repo.UpdateJobStatus(ctx, jobID, "FAILED"); markErr != nil {
			u.logger.Printf("failed to mark job %s as FAILED: %v", jobID, markErr)
		}
		return fmt.Errorf("upload file for job %s: %w", jobID, err)
	}

	if err := u.repo.UpdateJobAfterUpload(ctx, jobID, fileInfo.Name); err != nil {
		return fmt.Errorf("update job %s after upload: %w", jobID, err)
	}
	u.logger.Printf("uploaded file for job %s (provider ref: %s)", jobID, fileInfo.Name)

	nextTask, err := tasks.NewCheckStateTask(jobID, asynq.ProcessIn(10*time.Second))
	if err != nil {
		return fmt.Errorf("create check-state task for job %s: %w", jobID, err)
	}
	if _, err := u.asynqClient.Enqueue(nextTask); err != nil {
		return fmt.Errorf("enqueue check-state for job %s: %w", jobID, err)
	}
	u.logger.Printf("enqueued state check for job %s (10s delay)", jobID)

	return nil
}
