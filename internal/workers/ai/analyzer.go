package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

type Analyzer struct {
	repo        repository.RepositoryInterface
	gemini      gemini.GeminiInterface
	asynqClient tasks.AsynqEnqueuer
	outputDir   string
	logger      *log.Logger
}

func NewAnalyzer(repo repository.RepositoryInterface, geminiClient gemini.GeminiInterface, asynqClient tasks.AsynqEnqueuer, outputDir string, logger *log.Logger) *Analyzer {
	if outputDir == "" {
		outputDir = "./manifests"
	}
	return &Analyzer{
		repo:        repo,
		gemini:      geminiClient,
		asynqClient: asynqClient,
		outputDir:   outputDir,
		logger:      logger,
	}
}

func (a *Analyzer) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.JobPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	jobID := p.JobID

	a.logger.Printf("starting analysis for job %s", jobID)

	job, err := a.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", jobID, err)
	}

	if job.AIProviderFileID == nil || *job.AIProviderFileID == "" {
		a.logger.Printf("job %s has no ai_provider_file_id, marking FAILED", jobID)
		_ = a.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return nil
	}

	if err := a.repo.UpdateJobStatus(ctx, jobID, "ANALYZING"); err != nil {
		return fmt.Errorf("set ANALYZING for job %s: %w", jobID, err)
	}

	metadata, err := a.gemini.GenerateMetadata(ctx, *job.AIProviderFileID)
	if err != nil {
		a.logger.Printf("GenerateMetadata failed for job %s: %v", jobID, err)
		if metadata != nil {
			if wErr := a.writeManifest(jobID+"_broken", metadata); wErr != nil {
				a.logger.Printf("failed to write broken manifest for job %s: %v", jobID, wErr)
			} else {
				a.logger.Printf("saved broken manifest for job %s as %s_broken.jsonc", jobID, jobID)
			}
		}
		_ = a.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return fmt.Errorf("analyze job %s: %w", jobID, err)
	}

	if err := a.writeManifest(jobID, metadata); err != nil {
		a.logger.Printf("failed to write manifest for job %s: %v", jobID, err)
		_ = a.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return fmt.Errorf("write manifest for job %s: %w", jobID, err)
	}

	if err := a.repo.UpdateJobStatus(ctx, jobID, "READY_FOR_EMBEDDING"); err != nil {
		return fmt.Errorf("set READY_FOR_EMBEDDING for job %s: %w", jobID, err)
	}
	a.logger.Printf("analysis completed for job %s, now READY_FOR_EMBEDDING", jobID)

	nextTask, err := tasks.NewEmbedVideoTask(jobID)
	if err != nil {
		return fmt.Errorf("create embed task for job %s: %w", jobID, err)
	}
	if _, err := a.asynqClient.Enqueue(nextTask); err != nil {
		return fmt.Errorf("enqueue embed for job %s: %w", jobID, err)
	}
	a.logger.Printf("enqueued embedding for job %s", jobID)

	return nil
}

func (a *Analyzer) writeManifest(jobID string, metadata *gemini.GeminiMetadata) error {
	if err := os.MkdirAll(a.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	outPath := filepath.Join(a.outputDir, jobID+".jsonc")
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	a.logger.Printf("wrote manifest to %s", outPath)
	return nil
}
