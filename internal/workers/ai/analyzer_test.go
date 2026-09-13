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

func TestAnalyzer_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	fileID := "files/abc123"
	var enqueued []*asynq.Task
	var finalStatus string

	analyzer := NewAnalyzer(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, SourceVideoID: "vid-1", AIProviderFileID: &fileID, Status: "READY_FOR_ANALYSIS"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				finalStatus = status
				return nil
			},
		},
		&mockGemini{
			generateMetadata: func(_ context.Context, fileName string) (*gemini.GeminiMetadata, error) {
				return &gemini.GeminiMetadata{
					VideoTitle:    "Test Video",
					VideoCategory: "Tutorial",
					GlobalKeywords: []string{"test", "video"},
					Duration:      120.0,
					Scenes: []gemini.SceneMetadata{
						{StartTime: 0, EndTime: 60, Name: "Intro", Content: "Introduction scene", Items: []string{"camera"}},
						{StartTime: 60, EndTime: 120, Name: "Outro", Content: "Closing scene", Items: []string{"microphone"}},
					},
				}, nil
			},
		},
		&mockEnqueuer{enqueueFn: func(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
			enqueued = append(enqueued, task)
			return nil, nil
		}},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeAnalyzeVideo, payload)

	err := analyzer.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, finalStatus, "READY_FOR_EMBEDDING", "should transition to READY_FOR_EMBEDDING")
	assertEqual(t, len(enqueued), 1, "should enqueue embed task")
	assertEqual(t, enqueued[0].Type(), tasks.TypeEmbedVideo, "should enqueue embed task type")

	manifestData, readErr := os.ReadFile(tmpDir + "/job-1.jsonc")
	assertNoError(t, readErr)

	var manifest gemini.GeminiMetadata
	assertNoError(t, json.Unmarshal(manifestData, &manifest))
	assertEqual(t, manifest.VideoTitle, "Test Video", "manifest should contain video title")
	assertEqual(t, len(manifest.Scenes), 2, "manifest should contain 2 scenes")
}

func TestAnalyzer_SadPath_BadPayload(t *testing.T) {
	analyzer := NewAnalyzer(&mockRepo{}, &mockGemini{}, &mockEnqueuer{}, t.TempDir(), log.New(os.Stderr, "", 0))

	task := asynq.NewTask(tasks.TypeAnalyzeVideo, []byte("bad"))
	err := analyzer.ProcessTask(context.Background(), task)
	assertError(t, err, "unmarshal")
}

func TestAnalyzer_SadPath_NoFileID(t *testing.T) {
	var finalStatus string

	analyzer := NewAnalyzer(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, Status: "READY_FOR_ANALYSIS"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				finalStatus = status
				return nil
			},
		},
		&mockGemini{},
		&mockEnqueuer{},
		t.TempDir(),
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeAnalyzeVideo, payload)

	err := analyzer.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, finalStatus, "FAILED", "should mark FAILED when no file ID")
}

func TestAnalyzer_SadPath_GeminiFails(t *testing.T) {
	fileID := "files/abc123"
	var finalStatus string

	analyzer := NewAnalyzer(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: id, SourceVideoID: "vid-1", AIProviderFileID: &fileID, Status: "READY_FOR_ANALYSIS"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				if status == "FAILED" {
					finalStatus = status
				}
				return nil
			},
		},
		&mockGemini{
			generateMetadata: func(_ context.Context, _ string) (*gemini.GeminiMetadata, error) {
				return nil, errors.New("gemini quota exceeded")
			},
		},
		&mockEnqueuer{},
		t.TempDir(),
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := asynq.NewTask(tasks.TypeAnalyzeVideo, payload)

	err := analyzer.ProcessTask(context.Background(), task)
	assertError(t, err, "analyze job")
	assertEqual(t, finalStatus, "FAILED", "should mark FAILED on gemini error")
}
