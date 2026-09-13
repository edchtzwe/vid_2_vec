package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ingestion/internal/config"
	"ingestion/internal/repository"
)

type mockRepo struct {
	getSourceVideo   func(ctx context.Context, id string) (*repository.SourceVideo, error)
	createPendingJob func(ctx context.Context, videoID string, providerName string, modelName string) (string, error)
}

func (m *mockRepo) GetSourceVideo(ctx context.Context, id string) (*repository.SourceVideo, error) {
	return m.getSourceVideo(ctx, id)
}

func (m *mockRepo) CreatePendingJob(ctx context.Context, videoID string, providerName string, modelName string) (string, error) {
	return m.createPendingJob(ctx, videoID, providerName, modelName)
}

var _ RepoInterface = (*mockRepo)(nil)

func TestProcessor_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "vid-1.mp4")
	os.WriteFile(testFile, []byte("fake video"), 0644)

	processor := NewProcessor(
		&mockRepo{
			getSourceVideo: func(_ context.Context, id string) (*repository.SourceVideo, error) {
				return &repository.SourceVideo{ID: id}, nil
			},
			createPendingJob: func(_ context.Context, _ string, _ string, _ string) (string, error) {
				return "job-abc", nil
			},
		},
		&config.Config{
			SourceFileDir:          tmpDir,
			AIProvider:             "google",
			GoogleAIGenerativeModel: "gemini-2.5-flash",
		},
	)

	jobID, err := processor.StartIngestion(context.Background(), "vid-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if jobID != "job-abc" {
		t.Fatalf("expected job ID 'job-abc', got '%s'", jobID)
	}
}

func TestProcessor_SadPath_FileMissing(t *testing.T) {
	processor := NewProcessor(
		&mockRepo{},
		&config.Config{SourceFileDir: "/nonexistent"},
	)

	_, err := processor.StartIngestion(context.Background(), "vid-1")
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	if !contains(err.Error(), "file not found") {
		t.Fatalf("expected 'file not found' error, got: %v", err)
	}
}

func TestProcessor_SadPath_FileIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.Mkdir(filepath.Join(tmpDir, "vid-1.mp4"), 0755)

	processor := NewProcessor(
		&mockRepo{},
		&config.Config{SourceFileDir: tmpDir},
	)

	_, err := processor.StartIngestion(context.Background(), "vid-1")
	if err == nil {
		t.Fatal("expected error for directory")
	}

	if !contains(err.Error(), "directory") {
		t.Fatalf("expected 'directory' error, got: %v", err)
	}
}

func TestProcessor_SadPath_DBLookupFails(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "vid-1.mp4")
	os.WriteFile(testFile, []byte("fake video"), 0644)

	processor := NewProcessor(
		&mockRepo{
			getSourceVideo: func(_ context.Context, _ string) (*repository.SourceVideo, error) {
				return nil, errors.New("connection refused")
			},
		},
		&config.Config{SourceFileDir: tmpDir},
	)

	_, err := processor.StartIngestion(context.Background(), "vid-1")
	if err == nil {
		t.Fatal("expected error for DB failure")
	}

	if !contains(err.Error(), "database record not found") {
		t.Fatalf("expected 'database record not found' error, got: %v", err)
	}
}

func TestProcessor_SadPath_CreateJobFails(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "vid-1.mp4")
	os.WriteFile(testFile, []byte("fake video"), 0644)

	processor := NewProcessor(
		&mockRepo{
			getSourceVideo: func(_ context.Context, _ string) (*repository.SourceVideo, error) {
				return &repository.SourceVideo{ID: "vid-1"}, nil
			},
			createPendingJob: func(_ context.Context, _ string, _ string, _ string) (string, error) {
				return "", errors.New("duplicate key")
			},
		},
		&config.Config{SourceFileDir: tmpDir},
	)

	_, err := processor.StartIngestion(context.Background(), "vid-1")
	if err == nil {
		t.Fatal("expected error for job creation failure")
	}

	if !contains(err.Error(), "failed to create ingestion job") {
		t.Fatalf("expected 'failed to create ingestion job' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
