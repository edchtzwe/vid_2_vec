package service

import (
	"context"

	"ingestion/internal/repository"
)

type RepoInterface interface {
	GetSourceVideo(ctx context.Context, id string) (*repository.SourceVideo, error)
	CreatePendingJob(ctx context.Context, videoID string, providerName string, modelName string) (string, error)
}
