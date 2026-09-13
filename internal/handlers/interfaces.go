package handlers

import (
	"context"
)

type ProcessorInterface interface {
	StartIngestion(ctx context.Context, id string) (string, error)
}

type EmbedderInterface interface {
	ProcessJob(ctx context.Context, jobID string) error
}
