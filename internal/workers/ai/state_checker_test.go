package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/hibiken/asynq"

	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

func TestStateChecker_HappyPath_FileActive(t *testing.T) {
	var enqueued []*asynq.Task
	fileID := "files/abc123"

	checker := NewStateChecker(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, SourceVideoID: "vid-1", AIProviderFileID: &fileID, Status: "UPLOADED_TO_PROVIDER"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				assertEqual(t, status, "READY_FOR_ANALYSIS", "should transition to READY_FOR_ANALYSIS")
				return nil
			},
		},
		&mockGemini{
			getFileState: func(_ context.Context, name string) (string, error) {
				assertEqual(t, name, fileID, "should check correct file ID")
				return "ACTIVE", nil
			},
		},
		&mockEnqueuer{enqueueFn: func(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
			enqueued = append(enqueued, task)
			return nil, nil
		}},
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeCheckState, payload)

	err := checker.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, len(enqueued), 1, "should enqueue analyze task")
	assertEqual(t, enqueued[0].Type(), tasks.TypeAnalyzeVideo, "should enqueue analyze task type")
}

func TestStateChecker_SadPath_BadPayload(t *testing.T) {
	checker := NewStateChecker(&mockRepo{}, &mockGemini{}, &mockEnqueuer{}, log.New(os.Stderr, "", 0))

	task := asynq.NewTask(tasks.TypeCheckState, []byte("{bad"))
	err := checker.ProcessTask(context.Background(), task)
	assertError(t, err, "unmarshal")
}

func TestStateChecker_SadPath_NoFileID(t *testing.T) {
	var statusUpdated string

	checker := NewStateChecker(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, Status: "UPLOADED_TO_PROVIDER"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				statusUpdated = status
				return nil
			},
		},
		&mockGemini{},
		&mockEnqueuer{},
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeCheckState, payload)

	err := checker.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, statusUpdated, "FAILED", "should mark FAILED when no file ID")
}

func TestStateChecker_SadPath_FileFailed(t *testing.T) {
	var statusUpdated string
	fileID := "files/abc123"

	checker := NewStateChecker(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, AIProviderFileID: &fileID, Status: "UPLOADED_TO_PROVIDER"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				statusUpdated = status
				return nil
			},
		},
		&mockGemini{
			getFileState: func(_ context.Context, _ string) (string, error) {
				return "FAILED", nil
			},
		},
		&mockEnqueuer{},
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeCheckState, payload)

	err := checker.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, statusUpdated, "FAILED", "should mark FAILED when provider reports FAILED")
}

func TestStateChecker_SadPath_ProcessingReturnsError(t *testing.T) {
	fileID := "files/abc123"

	checker := NewStateChecker(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, AIProviderFileID: &fileID, Status: "UPLOADED_TO_PROVIDER"}, nil
			},
		},
		&mockGemini{
			getFileState: func(_ context.Context, _ string) (string, error) {
				return "PROCESSING", nil
			},
		},
		&mockEnqueuer{},
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeCheckState, payload)

	err := checker.ProcessTask(context.Background(), task)
	assertError(t, err, "PROCESSING")
}

func TestStateChecker_SadPath_GetStateFails(t *testing.T) {
	fileID := "files/abc123"

	checker := NewStateChecker(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, AIProviderFileID: &fileID, Status: "UPLOADED_TO_PROVIDER"}, nil
			},
		},
		&mockGemini{
			getFileState: func(_ context.Context, _ string) (string, error) {
				return "", errors.New("api unreachable")
			},
		},
		&mockEnqueuer{},
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeCheckState, payload)

	err := checker.ProcessTask(context.Background(), task)
	assertError(t, err, "get file state")
}
