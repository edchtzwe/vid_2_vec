package repository

import "context"

type RepositoryInterface interface {
	GetJobByID(ctx context.Context, id string) (*IngestionJob, error)
	UpdateJobStatus(ctx context.Context, id string, status string) error
	UpdateJobAfterUpload(ctx context.Context, id string, providerFileID string) error
	GetSourceVideo(ctx context.Context, id string) (*SourceVideo, error)
	CreatePendingJob(ctx context.Context, videoID string, providerName string, modelName string) (string, error)
	CreateVideoScene(ctx context.Context, params CreateVideoSceneParams) (string, error)
	UpdateVideoScene(ctx context.Context, params UpdateVideoSceneParams) error
	DeleteScenesBySourceVideoID(ctx context.Context, sourceVideoID string) error
	GetScenesByVideoID(ctx context.Context, videoID string) ([]VideoScene, error)
	BulkReplaceScenes(ctx context.Context, sourceVideoID string, scenes []CreateVideoSceneParams) error
	SyncScenes(ctx context.Context, sourceVideoID string, toCreate []CreateVideoSceneParams, toUpdate []UpdateVideoSceneParams, toDeleteIDs []string) error
}
