package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

func TestUploader_HappyPath(t *testing.T) {
	var enqueued []*asynq.Task

	uploader := NewUploader(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, SourceVideoID: "vid-1", Status: "PENDING"}, nil
			},
			updateJobAfterUpload: func(_ context.Context, id string, providerFileID string) error {
				return nil
			},
		},
		&mockGemini{
			uploadFile: func(_ context.Context, path string) (*gemini.FileInfo, error) {
				return &gemini.FileInfo{Name: "files/abc123", URI: "files/abc123"}, nil
			},
		},
		&mockEnqueuer{enqueueFn: func(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
			enqueued = append(enqueued, nil)
			return nil, nil
		}},
		"/data/source",
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeUploadVideo, payload)

	err := uploader.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, len(enqueued), 1, "should enqueue state check task")
}

func TestUploader_SadPath_BadPayload(t *testing.T) {
	uploader := NewUploader(
		&mockRepo{},
		&mockGemini{},
		&mockEnqueuer{},
		"/data",
		log.New(os.Stderr, "", 0),
	)

	task := asynq.NewTask(tasks.TypeUploadVideo, []byte("not-json"))
	err := uploader.ProcessTask(context.Background(), task)
	assertError(t, err, "unmarshal")
}

func TestUploader_SadPath_JobNotFound(t *testing.T) {
	uploader := NewUploader(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return nil, errors.New("job not found")
			},
		},
		&mockGemini{},
		&mockEnqueuer{},
		"/data",
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "missing-job"})
	task := asynq.NewTask(tasks.TypeUploadVideo, payload)

	err := uploader.ProcessTask(context.Background(), task)
	assertError(t, err, "get job")
}

func TestUploader_SadPath_UploadFails(t *testing.T) {
	var statusUpdated string

	uploader := NewUploader(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, SourceVideoID: "vid-1", Status: "PENDING"}, nil
			},
			updateJobStatus: func(_ context.Context, id string, status string) error {
				statusUpdated = status
				return nil
			},
		},
		&mockGemini{
			uploadFile: func(_ context.Context, _ string) (*gemini.FileInfo, error) {
				return nil, errors.New("google api timeout")
			},
		},
		&mockEnqueuer{},
		"/data",
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeUploadVideo, payload)

	err := uploader.ProcessTask(context.Background(), task)
	assertError(t, err, "upload file")
	assertEqual(t, statusUpdated, "FAILED", "should mark job FAILED on upload error")
}
