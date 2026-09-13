package handlers

import (
	"context"

	"ingestion/internal/repository"
	"ingestion/internal/tasks"

	"github.com/hibiken/asynq"
)

type mockProcessor struct {
	startIngestion func(ctx context.Context, id string) (string, error)
}

func (m *mockProcessor) StartIngestion(ctx context.Context, id string) (string, error) {
	return m.startIngestion(ctx, id)
}

type mockRepo struct {
	getJobByID func(ctx context.Context, id string) (*repository.IngestionJob, error)
}

func (m *mockRepo) GetJobByID(ctx context.Context, id string) (*repository.IngestionJob, error) {
	return m.getJobByID(ctx, id)
}

func (m *mockRepo) UpdateJobStatus(ctx context.Context, id string, status string) error {
	return nil
}

func (m *mockRepo) UpdateJobAfterUpload(ctx context.Context, id string, providerFileID string) error {
	return nil
}

func (m *mockRepo) GetSourceVideo(ctx context.Context, id string) (*repository.SourceVideo, error) {
	return nil, nil
}

func (m *mockRepo) CreatePendingJob(ctx context.Context, videoID string, providerName string, modelName string) (string, error) {
	return "", nil
}

func (m *mockRepo) CreateVideoScene(ctx context.Context, params repository.CreateVideoSceneParams) (string, error) {
	return "", nil
}

func (m *mockRepo) UpdateVideoScene(ctx context.Context, params repository.UpdateVideoSceneParams) error {
	return nil
}

func (m *mockRepo) DeleteScenesBySourceVideoID(ctx context.Context, sourceVideoID string) error {
	return nil
}

func (m *mockRepo) GetScenesByVideoID(ctx context.Context, videoID string) ([]repository.VideoScene, error) {
	return nil, nil
}

func (m *mockRepo) BulkReplaceScenes(ctx context.Context, sourceVideoID string, scenes []repository.CreateVideoSceneParams) error {
	return nil
}

func (m *mockRepo) SyncScenes(ctx context.Context, sourceVideoID string, toCreate []repository.CreateVideoSceneParams, toUpdate []repository.UpdateVideoSceneParams, toDeleteIDs []string) error {
	return nil
}

type mockEmbedder struct {
	processJob func(ctx context.Context, jobID string) error
}

func (m *mockEmbedder) ProcessJob(ctx context.Context, jobID string) error {
	return m.processJob(ctx, jobID)
}

type mockEnqueuer struct {
	enqueueFn func(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

func (m *mockEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return m.enqueueFn(task, opts...)
}

var _ ProcessorInterface = (*mockProcessor)(nil)
var _ repository.RepositoryInterface = (*mockRepo)(nil)
var _ EmbedderInterface = (*mockEmbedder)(nil)
var _ tasks.AsynqEnqueuer = (*mockEnqueuer)(nil)
