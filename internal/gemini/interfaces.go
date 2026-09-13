package gemini

import "context"

type GeminiInterface interface {
	UploadFile(ctx context.Context, path string) (*FileInfo, error)
	GetFileState(ctx context.Context, name string) (string, error)
	GenerateMetadata(ctx context.Context, fileName string) (*GeminiMetadata, error)
	GenerateEmbeddingWithRetry(ctx context.Context, text string, maxAttempts int) (string, error)
}
