package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

func writeManifest(t *testing.T, dir string, jobID string, metadata *gemini.GeminiMetadata) {
	t.Helper()
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(dir+"/"+jobID+".jsonc", data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestEmbedder_HappyPath_InitialIngestion(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := &gemini.GeminiMetadata{
		VideoTitle:    "Test Video",
		VideoCategory: "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:      60.0,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 30, Name: "Scene 1", Content: "First scene", Items: []string{"item1"}},
			{StartTime: 30, EndTime: 60, Name: "Scene 2", Content: "Second scene", Items: []string{"item2"}},
		},
	}
	writeManifest(t, tmpDir, "job-1", manifest)

	var savedScenes []repository.CreateVideoSceneParams
	var finalStatus string

	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "READY_FOR_EMBEDDING", AIProviderName: "google", AIModelName: "gemini-2.5-flash"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				finalStatus = status
				return nil
			},
			bulkReplaceScenes: func(_ context.Context, _ string, scenes []repository.CreateVideoSceneParams) error {
				savedScenes = scenes
				return nil
			},
		},
		&mockGemini{
			generateEmbeddingWithRetry: func(_ context.Context, text string, _ int) (string, error) {
				return "[0.1, 0.2, 0.3]", nil
			},
		},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, finalStatus, "COMPLETED", "should transition to COMPLETED")
	assertEqual(t, len(savedScenes), 2, "should save 2 scenes")
	assertEqual(t, savedScenes[0].SourceVideoID, "vid-1", "scene should reference source video")
	assertEqual(t, savedScenes[0].AIProvider, "google", "scene should record AI provider")
}

func TestEmbedder_SadPath_BadPayload(t *testing.T) {
	embedder := NewEmbedder(&mockRepo{}, &mockGemini{}, t.TempDir(), log.New(os.Stderr, "", 0))

	task := newTestTask(tasks.TypeEmbedVideo, []byte("{bad"))
	err := embedder.ProcessTask(context.Background(), task)
	assertError(t, err, "unmarshal")
}

func TestEmbedder_SadPath_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := &gemini.GeminiMetadata{
		VideoTitle:    "Empty Video",
		VideoCategory: "Test",
		GlobalKeywords: []string{"empty"},
		Duration:      60.0,
		Scenes:        []gemini.SceneMetadata{},
	}
	writeManifest(t, tmpDir, "job-1", manifest)

	var finalStatus string

	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "READY_FOR_EMBEDDING"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				finalStatus = status
				return nil
			},
		},
		&mockGemini{},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, finalStatus, "FAILED", "should mark FAILED for empty scenes")
}

func TestEmbedder_SadPath_EmbeddingFails(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := &gemini.GeminiMetadata{
		VideoTitle:    "Test Video",
		VideoCategory: "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:      60.0,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Scene 1", Content: "A scene", Items: []string{"item1"}},
		},
	}
	writeManifest(t, tmpDir, "job-1", manifest)

	var finalStatus string

	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "READY_FOR_EMBEDDING"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				if status == "FAILED" {
					finalStatus = status
				}
				return nil
			},
		},
		&mockGemini{
			generateEmbeddingWithRetry: func(_ context.Context, _ string, _ int) (string, error) {
				return "", errors.New("embedding API down")
			},
		},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(context.Background(), task)
	assertError(t, err, "embed scene")
	assertEqual(t, finalStatus, "FAILED", "should mark FAILED on embedding error")
}

func TestEmbedder_SadPath_MissingManifest(t *testing.T) {
	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "READY_FOR_EMBEDDING"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, _ string) error {
				return nil
			},
		},
		&mockGemini{},
		t.TempDir(),
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(context.Background(), task)
	assertError(t, err, "read manifest")
}

func TestEmbedder_HappyPath_Reembedding(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := &gemini.GeminiMetadata{
		VideoTitle:    "Test Video",
		VideoCategory: "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:      60.0,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 30, Name: "Scene 1", Content: "Same content", Items: []string{"item1"}},
			{StartTime: 30, EndTime: 60, Name: "Scene 2", Content: "Updated content", Items: []string{"item2"}},
		},
	}
	writeManifest(t, tmpDir, "job-1", manifest)

	unchangedSoup := gemini.BuildDenseContent(manifest, &manifest.Scenes[0])
	existingScenes := []repository.VideoScene{
		{ID: "scene-1", SourceVideoID: "vid-1", StartTime: 0, EndTime: 30, Content: "Same content", Name: "Scene 1", ContentHash: hashContent(unchangedSoup)},
		{ID: "scene-2", SourceVideoID: "vid-1", StartTime: 30, EndTime: 60, Content: "Old content", Name: "Scene 2", ContentHash: "stale-hash"},
	}

	var syncedCreates []repository.CreateVideoSceneParams
	var syncedUpdates []repository.UpdateVideoSceneParams
	var syncedDeletes []string
	var finalStatus string

	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "COMPLETED", AIProviderName: "google", AIModelName: "gemini-2.5-flash"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				finalStatus = status
				return nil
			},
			getScenesByVideoID: func(_ context.Context, _ string) ([]repository.VideoScene, error) {
				return existingScenes, nil
			},
			syncScenes: func(_ context.Context, _ string, creates []repository.CreateVideoSceneParams, updates []repository.UpdateVideoSceneParams, deletes []string) error {
				syncedCreates = creates
				syncedUpdates = updates
				syncedDeletes = deletes
				return nil
			},
		},
		&mockGemini{
			generateEmbeddingWithRetry: func(_ context.Context, _ string, _ int) (string, error) {
				return "[0.1, 0.2, 0.3]", nil
			},
		},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(context.Background(), task)
	assertNoError(t, err)
	assertEqual(t, finalStatus, "COMPLETED", "should transition back to COMPLETED")
	assertEqual(t, len(syncedUpdates), 1, "should update 1 changed scene")
	assertEqual(t, syncedUpdates[0].ID, "scene-2", "should update the stale scene")
	assertEqual(t, len(syncedCreates), 0, "should create 0 new scenes")
	assertEqual(t, len(syncedDeletes), 0, "should delete 0 scenes")
}

func TestEmbedder_SadPath_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := &gemini.GeminiMetadata{
		VideoTitle:    "Test Video",
		VideoCategory: "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:      120.0,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Scene 1", Content: "First", Items: []string{"a"}},
			{StartTime: 60, EndTime: 120, Name: "Scene 2", Content: "Second", Items: []string{"b"}},
		},
	}
	writeManifest(t, tmpDir, "job-1", manifest)

	callCount := 0
	var finalStatus string

	embedder := NewEmbedder(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{ID: "job-1", SourceVideoID: "vid-1", Status: "READY_FOR_EMBEDDING", AIProviderName: "google", AIModelName: "gemini-2.5-flash"}, nil
			},
			updateJobStatus: func(_ context.Context, _ string, status string) error {
				if status == "FAILED" {
					finalStatus = status
				}
				return nil
			},
			bulkReplaceScenes: func(_ context.Context, _ string, _ []repository.CreateVideoSceneParams) error {
				return nil
			},
		},
		&mockGemini{
			generateEmbeddingWithRetry: func(ctx context.Context, _ string, _ int) (string, error) {
				callCount++
				if callCount > 1 {
					return "", ctx.Err()
				}
				return "[0.1, 0.2]", nil
			},
		},
		tmpDir,
		log.New(os.Stderr, "", 0),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	payload, _ := json.Marshal(tasks.JobPayload{JobID: "job-1"})
	task := newTestTask(tasks.TypeEmbedVideo, payload)

	err := embedder.ProcessTask(ctx, task)
	assertError(t, err, "context")
	assertEqual(t, finalStatus, "FAILED", "should mark FAILED on context cancellation")
}
