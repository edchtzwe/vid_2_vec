package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

type StateChecker struct {
	repo        repository.RepositoryInterface
	gemini      gemini.GeminiInterface
	asynqClient tasks.AsynqEnqueuer
	logger      *log.Logger
}

func NewStateChecker(repo repository.RepositoryInterface, geminiClient gemini.GeminiInterface, asynqClient tasks.AsynqEnqueuer, logger *log.Logger) *StateChecker {
	return &StateChecker{
		repo:        repo,
		gemini:      geminiClient,
		asynqClient: asynqClient,
		logger:      logger,
	}
}

func (s *StateChecker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.JobPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	jobID := p.JobID

	job, err := s.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", jobID, err)
	}

	if job.AIProviderFileID == nil || *job.AIProviderFileID == "" {
		s.logger.Printf("job %s has no provider file ID, marking FAILED", jobID)
		_ = s.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return nil
	}

	state, err := s.gemini.GetFileState(ctx, *job.AIProviderFileID)
	if err != nil {
		return fmt.Errorf("get file state for job %s: %w", jobID, err)
	}

	switch state {
	case "ACTIVE":
		if err := s.repo.UpdateJobStatus(ctx, jobID, "READY_FOR_ANALYSIS"); err != nil {
			return fmt.Errorf("mark job %s READY_FOR_ANALYSIS: %w", jobID, err)
		}
		s.logger.Printf("job %s file is ACTIVE, transitioning to READY_FOR_ANALYSIS", jobID)

		nextTask, err := tasks.NewAnalyzeVideoTask(jobID)
		if err != nil {
			return fmt.Errorf("create analyze task for job %s: %w", jobID, err)
		}
		if _, err := s.asynqClient.Enqueue(nextTask); err != nil {
			return fmt.Errorf("enqueue analyze for job %s: %w", jobID, err)
		}
		s.logger.Printf("enqueued analysis for job %s", jobID)
		return nil

	case "FAILED":
		s.logger.Printf("job %s file FAILED at provider", jobID)
		_ = s.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return nil

	default:
		// PROCESSING or any other transient state — return an error so Asynq retries.
		s.logger.Printf("job %s file state: %s, will retry", jobID, state)
		return fmt.Errorf("job %s file still %s", jobID, state)
	}
}
