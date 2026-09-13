package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
)

type mockRepo struct {
	getJobByID               func(ctx context.Context, id string) (*repository.IngestionJob, error)
	updateJobStatus          func(ctx context.Context, id string, status string) error
	updateJobAfterUpload     func(ctx context.Context, id string, providerFileID string) error
	getSourceVideo           func(ctx context.Context, id string) (*repository.SourceVideo, error)
	createPendingJob         func(ctx context.Context, videoID string, providerName string, modelName string) (string, error)
	createVideoScene         func(ctx context.Context, params repository.CreateVideoSceneParams) (string, error)
	updateVideoScene         func(ctx context.Context, params repository.UpdateVideoSceneParams) error
	deleteScenesBySourceID   func(ctx context.Context, sourceVideoID string) error
	getScenesByVideoID       func(ctx context.Context, videoID string) ([]repository.VideoScene, error)
	bulkReplaceScenes        func(ctx context.Context, sourceVideoID string, scenes []repository.CreateVideoSceneParams) error
	syncScenes               func(ctx context.Context, sourceVideoID string, toCreate []repository.CreateVideoSceneParams, toUpdate []repository.UpdateVideoSceneParams, toDeleteIDs []string) error
}

func (m *mockRepo) GetJobByID(ctx context.Context, id string) (*repository.IngestionJob, error) {
	return m.getJobByID(ctx, id)
}

func (m *mockRepo) UpdateJobStatus(ctx context.Context, id string, status string) error {
	if m.updateJobStatus != nil {
		return m.updateJobStatus(ctx, id, status)
	}
	return nil
}

func (m *mockRepo) UpdateJobAfterUpload(ctx context.Context, id string, providerFileID string) error {
	if m.updateJobAfterUpload != nil {
		return m.updateJobAfterUpload(ctx, id, providerFileID)
	}
	return nil
}

func (m *mockRepo) GetSourceVideo(ctx context.Context, id string) (*repository.SourceVideo, error) {
	return m.getSourceVideo(ctx, id)
}

func (m *mockRepo) CreatePendingJob(ctx context.Context, videoID string, providerName string, modelName string) (string, error) {
	return m.createPendingJob(ctx, videoID, providerName, modelName)
}

func (m *mockRepo) CreateVideoScene(ctx context.Context, params repository.CreateVideoSceneParams) (string, error) {
	return m.createVideoScene(ctx, params)
}

func (m *mockRepo) UpdateVideoScene(ctx context.Context, params repository.UpdateVideoSceneParams) error {
	return m.updateVideoScene(ctx, params)
}

func (m *mockRepo) DeleteScenesBySourceVideoID(ctx context.Context, sourceVideoID string) error {
	return m.deleteScenesBySourceID(ctx, sourceVideoID)
}

func (m *mockRepo) GetScenesByVideoID(ctx context.Context, videoID string) ([]repository.VideoScene, error) {
	return m.getScenesByVideoID(ctx, videoID)
}

func (m *mockRepo) BulkReplaceScenes(ctx context.Context, sourceVideoID string, scenes []repository.CreateVideoSceneParams) error {
	if m.bulkReplaceScenes != nil {
		return m.bulkReplaceScenes(ctx, sourceVideoID, scenes)
	}
	return nil
}

func (m *mockRepo) SyncScenes(ctx context.Context, sourceVideoID string, toCreate []repository.CreateVideoSceneParams, toUpdate []repository.UpdateVideoSceneParams, toDeleteIDs []string) error {
	if m.syncScenes != nil {
		return m.syncScenes(ctx, sourceVideoID, toCreate, toUpdate, toDeleteIDs)
	}
	return nil
}

type mockGemini struct {
	uploadFile                func(ctx context.Context, path string) (*gemini.FileInfo, error)
	getFileState              func(ctx context.Context, name string) (string, error)
	generateMetadata          func(ctx context.Context, fileName string) (*gemini.GeminiMetadata, error)
	generateEmbeddingWithRetry func(ctx context.Context, text string, maxAttempts int) (string, error)
}

func (m *mockGemini) UploadFile(ctx context.Context, path string) (*gemini.FileInfo, error) {
	return m.uploadFile(ctx, path)
}

func (m *mockGemini) GetFileState(ctx context.Context, name string) (string, error) {
	return m.getFileState(ctx, name)
}

func (m *mockGemini) GenerateMetadata(ctx context.Context, fileName string) (*gemini.GeminiMetadata, error) {
	return m.generateMetadata(ctx, fileName)
}

func (m *mockGemini) GenerateEmbeddingWithRetry(ctx context.Context, text string, maxAttempts int) (string, error) {
	return m.generateEmbeddingWithRetry(ctx, text, maxAttempts)
}

type mockEnqueuer struct {
	enqueueFn func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

func (m *mockEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return m.enqueueFn(task, opts...)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func assertError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", contains)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected error containing %q, got: %v", contains, err)
	}
}

func assertEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}

func newTestTask(taskType string, payload []byte) *asynq.Task {
	return asynq.NewTask(taskType, payload)
}
