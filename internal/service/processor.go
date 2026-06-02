package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ingestion/internal/config"
	"ingestion/internal/repository"
)

type Processor struct {
	repo       *repository.Repository
	sourceDir  string
	aiProvider string
	aiModel    string
}

func NewProcessor(repo *repository.Repository, cfg *config.Config) *Processor {
	return &Processor{
		repo:       repo,
		sourceDir:  cfg.SourceFileDir,
		aiProvider: cfg.AIProvider,
		aiModel:    cfg.GoogleAIGenerativeModel,
	}
}

// StartIngestion validates the source file and DB record, then creates a PENDING
// job for the background worker to pick up. Returns the job ID immediately.
func (p *Processor) StartIngestion(ctx context.Context, id string) (string, error) {
	filename := id + ".mp4"
	fullPath := filepath.Join(p.sourceDir, filename)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found on disk: %s", fullPath)
		}
		return "", fmt.Errorf("error checking file system: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", fullPath)
	}

	_, err = p.repo.GetSourceVideo(ctx, id)
	if err != nil {
		return "", fmt.Errorf("database record not found for id %s: %w", id, err)
	}

	jobID, err := p.repo.CreatePendingJob(ctx, id, p.aiProvider, p.aiModel)
	if err != nil {
		return "", fmt.Errorf("failed to create ingestion job: %w", err)
	}

	return jobID, nil
}