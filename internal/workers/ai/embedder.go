package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/tasks"
)

type Embedder struct {
	repo      repository.RepositoryInterface
	gemini    gemini.GeminiInterface
	outputDir string
	logger    *log.Logger
}

func NewEmbedder(repo repository.RepositoryInterface, geminiClient gemini.GeminiInterface, outputDir string, logger *log.Logger) *Embedder {
	if outputDir == "" {
		panic("outputDir is required and cannot be empty")
	}
	return &Embedder{
		repo:      repo,
		gemini:    geminiClient,
		outputDir: outputDir,
		logger:    logger,
	}
}

// ProcessTask satisfies asynq.Handler. It unmarshals the task payload and
// delegates all real work to ProcessJob.
func (e *Embedder) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.JobPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	return e.ProcessJob(ctx, p.JobID)
}

// ProcessJob contains the full embedding workflow. It can be called from the
// Asynq worker (via ProcessTask) or directly from an HTTP handler.
func (e *Embedder) ProcessJob(ctx context.Context, jobID string) error {
	e.logger.Printf("starting embedding for job %s", jobID)

	job, err := e.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job %s: %w", jobID, err)
	}

	switch job.Status {
	case "READY_FOR_EMBEDDING":
		return e.processInitialIngestion(ctx, job)
	case "COMPLETED":
		return e.processReembedding(ctx, job)
	default:
		e.logger.Printf("job %s has status %q — not actionable for embedding, skipping", jobID, job.Status)
		return nil
	}
}

// processInitialIngestion handles PATH A: fresh ingestion from a manifest.
// CHANGE: Refactored to use repo.BulkReplaceScenes instead of inline DB loops.
func (e *Embedder) processInitialIngestion(ctx context.Context, job *repository.IngestionJob) error {
	jobID := job.ID

	if err := e.repo.UpdateJobStatus(ctx, jobID, "EMBEDDING"); err != nil {
		return fmt.Errorf("set EMBEDDING for job %s: %w", jobID, err)
	}

	manifest, err := e.loadManifest(jobID)
	if err != nil {
		return e.failJob(ctx, jobID, "load manifest for job %s: %w", jobID, err)
	}

	if len(manifest.Scenes) == 0 {
		e.logger.Printf("manifest for job %s has no scenes, marking FAILED", jobID)
		_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return nil
	}

	e.logger.Printf("processing %d scenes for job %s (initial ingestion)", len(manifest.Scenes), jobID)

	var newScenes[]repository.CreateVideoSceneParams

	for i, scene := range manifest.Scenes {
		if ctx.Err() != nil {
			_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
			return ctx.Err()
		}

		denseSoup := gemini.BuildDenseContent(manifest, &scene)
		embedding, err := e.embedScene(ctx, jobID, i, len(manifest.Scenes), denseSoup)
		if err != nil {
			return e.failJob(ctx, jobID, "embed scene %d for job %s: %w", i+1, jobID, err)
		}

		newScenes = append(newScenes, repository.CreateVideoSceneParams{
			SourceVideoID: job.SourceVideoID,
			StartTime:     scene.StartTime,
			EndTime:       scene.EndTime,
			Content:       scene.Content,
			Name:          scene.Name,
			ContentHash:   hashContent(denseSoup),
			Embedding:     embedding,
			AIProvider:    job.AIProviderName,
			AIModel:       job.AIModelName,
		})

		e.logger.Printf("embedded scene %d/%d for job %s (%.1fs–%.1fs)",
			i+1, len(manifest.Scenes), jobID, scene.StartTime, scene.EndTime)
	}

	if err := e.repo.BulkReplaceScenes(ctx, job.SourceVideoID, newScenes); err != nil {
		return e.failJob(ctx, jobID, "bulk save scenes for job %s: %w", jobID, err)
	}

	if err := e.repo.UpdateJobStatus(ctx, jobID, "COMPLETED"); err != nil {
		return fmt.Errorf("set COMPLETED for job %s: %w", jobID, err)
	}

	e.logger.Printf("initial embedding completed for job %s — %d scenes stored", jobID, len(manifest.Scenes))
	return nil
}

// processReembedding handles PATH B: selective re-embedding of an already-completed job.
// CHANGE: Refactored to use planReembedding (from planner.go) and repo.SyncScenes.
func (e *Embedder) processReembedding(ctx context.Context, job *repository.IngestionJob) error {
	jobID := job.ID

	if err := e.repo.UpdateJobStatus(ctx, jobID, "MANUAL_EMBEDDING"); err != nil {
		return fmt.Errorf("set MANUAL_EMBEDDING for job %s: %w", jobID, err)
	}

	manifest, err := e.loadManifest(jobID)
	if err != nil {
		return e.failJob(ctx, jobID, "load manifest for job %s: %w", jobID, err)
	}

	if len(manifest.Scenes) == 0 {
		e.logger.Printf("manifest for job %s has no scenes, marking FAILED", jobID)
		_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
		return nil
	}

	existingScenes, err := e.repo.GetScenesByVideoID(ctx, job.SourceVideoID)
	if err != nil {
		return e.failJob(ctx, jobID, "fetch existing scenes for job %s: %w", jobID, err)
	}

	plan := planReembedding(manifest, existingScenes)

	e.logger.Printf("re-embedding plan for job %s: %d to create, %d to update, %d unchanged, %d to delete",
		jobID, len(plan.ToCreate), len(plan.ToUpdate), plan.UnchangedCount, len(plan.ToDeleteIDs))

	var scenesToCreate[]repository.CreateVideoSceneParams
	var scenesToUpdate[]repository.UpdateVideoSceneParams

	for i, item := range plan.ToUpdate {
		if ctx.Err() != nil {
			_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
			return ctx.Err()
		}

		embedding, err := e.embedScene(ctx, jobID, i, len(plan.ToUpdate), item.DenseSoup)
		if err != nil {
			return e.failJob(ctx, jobID, "re-embed scene for job %s: %w", jobID, err)
		}

		scenesToUpdate = append(scenesToUpdate, repository.UpdateVideoSceneParams{
			ID:          item.ExistingID,
			Content:     item.Scene.Content,
			Name:        item.Scene.Name,
			ContentHash: item.NewHash,
			Embedding:   embedding,
			AIProvider:  job.AIProviderName,
			AIModel:     job.AIModelName,
		})
	}

	for i, item := range plan.ToCreate {
		if ctx.Err() != nil {
			_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
			return ctx.Err()
		}

		embedding, err := e.embedScene(ctx, jobID, i, len(plan.ToCreate), item.DenseSoup)
		if err != nil {
			return e.failJob(ctx, jobID, "embed new scene for job %s: %w", jobID, err)
		}

		scenesToCreate = append(scenesToCreate, repository.CreateVideoSceneParams{
			SourceVideoID: job.SourceVideoID,
			StartTime:     item.Scene.StartTime,
			EndTime:       item.Scene.EndTime,
			Content:       item.Scene.Content,
			Name:          item.Scene.Name,
			ContentHash:   item.NewHash,
			Embedding:     embedding,
			AIProvider:    job.AIProviderName,
			AIModel:       job.AIModelName,
		})
	}

	if err := e.repo.SyncScenes(ctx, job.SourceVideoID, scenesToCreate, scenesToUpdate, plan.ToDeleteIDs); err != nil {
		return e.failJob(ctx, jobID, "sync scenes for job %s: %w", jobID, err)
	}

	if err := e.repo.UpdateJobStatus(ctx, jobID, "COMPLETED"); err != nil {
		return fmt.Errorf("set COMPLETED for job %s: %w", jobID, err)
	}

	e.logger.Printf("re-embedding completed for job %s", jobID)
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// embedScene calls Gemini and parses the resulting vector.
// CHANGE: Now uses the gemini.Client's GenerateEmbeddingWithRetry method.
func (e *Embedder) embedScene(ctx context.Context, jobID string, idx, total int, content string) ([]float64, error) {
	raw, err := e.gemini.GenerateEmbeddingWithRetry(ctx, content, 3)
	if err != nil {
		e.logger.Printf("embedding failed for job %s scene %d/%d: %v", jobID, idx+1, total, err)
		return nil, err
	}

	var vec[]float64
	if err := json.Unmarshal([]byte(raw), &vec); err != nil {
		return nil, fmt.Errorf("parse embedding for job %s scene %d: %w", jobID, idx+1, err)
	}
	return vec, nil
}

// failJob sets the job status to FAILED and returns a formatted error.
func (e *Embedder) failJob(ctx context.Context, jobID string, format string, args ...any) error {
	_ = e.repo.UpdateJobStatus(ctx, jobID, "FAILED")
	return fmt.Errorf(format, args...)
}

func (e *Embedder) loadManifest(jobID string) (*gemini.GeminiMetadata, error) {
	path := filepath.Join(e.outputDir, jobID+".jsonc")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}

	var manifest gemini.GeminiMetadata
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}

	return &manifest, nil
}

// Ensure Embedder satisfies asynq.Handler at compile time.
var _ asynq.Handler = (*Embedder)(nil)

// Compile-time interface checks for the other handlers in this package.
var _ asynq.Handler = (*Uploader)(nil)
var _ asynq.Handler = (*StateChecker)(nil)
var _ asynq.Handler = (*Analyzer)(nil)