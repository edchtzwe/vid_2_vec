package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IngestionJob represents the tracking state of a video analysis job.
type IngestionJob struct {
	ID               string    `db:"id" json:"id"`
	SourceVideoID    string    `db:"source_video_id" json:"source_video_id"`
	AIProviderFileID *string   `db:"ai_provider_file_id" json:"ai_provider_file_id"`
	AIProviderName   string    `db:"ai_provider_name" json:"ai_provider_name"`
	AIModelName      string    `db:"ai_model_name" json:"ai_model_name"`
	Status           string    `db:"status" json:"status"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// CreatePendingJob inserts a job with status PENDING before the file is uploaded.
// Returns the generated UUID so the caller can respond immediately.
func (r *Repository) CreatePendingJob(
	ctx context.Context,
	videoID string,
	providerName string,
	modelName string,
) (string, error) {
	newID := uuid.New().String()

	query := fmt.Sprintf(`
		INSERT INTO %s.ingestion_job 
		(id, source_video_id, ai_provider_name, ai_model_name, status) 
		VALUES ($1, $2, $3, $4, 'PENDING')`, r.schema)

	_, err := r.pool.Exec(ctx, query, newID, videoID, providerName, modelName)
	if err != nil {
		return "", fmt.Errorf("create pending job: %w", err)
	}

	return newID, nil
}

// GetPendingJobs finds jobs that need work (upload or analysis).
func (r *Repository) GetPendingJobs(ctx context.Context) ([]IngestionJob, error) {
	query := fmt.Sprintf(`
		SELECT id, source_video_id, ai_provider_file_id, ai_provider_name, ai_model_name, status, created_at, updated_at 
		FROM %s.ingestion_job 
		WHERE status IN ('PENDING', 'UPLOADED_TO_PROVIDER', 'PROVIDER_PROCESSING')`, r.schema)

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []IngestionJob
	for rows.Next() {
		var j IngestionJob
		err := rows.Scan(
			&j.ID,
			&j.SourceVideoID,
			&j.AIProviderFileID,
			&j.AIProviderName,
			&j.AIModelName,
			&j.Status,
			&j.CreatedAt,
			&j.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	
	// Check for errors after iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return jobs, nil
}

// GetJobByID retrieves a single ingestion job by its ID.
func (r *Repository) GetJobByID(ctx context.Context, id string) (*IngestionJob, error) {
	query := fmt.Sprintf(`
		SELECT id, source_video_id, ai_provider_file_id, ai_provider_name,
		       ai_model_name, status, created_at, updated_at
		FROM %s.ingestion_job WHERE id = $1`, r.schema)

	var j IngestionJob
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&j.ID, &j.SourceVideoID, &j.AIProviderFileID,
		&j.AIProviderName, &j.AIModelName, &j.Status,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get job by id: %w", err)
	}
	return &j, nil
}

// UpdateJobStatus updates the state (e.g., to READY or FAILED).
func (r *Repository) UpdateJobStatus(ctx context.Context, id string, status string) error {
	query := fmt.Sprintf(`UPDATE %s.ingestion_job SET status = $1 WHERE id = $2`, r.schema)

	_, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

// UpdateJobAfterUpload sets the provider file ID and transitions status to UPLOADED_TO_PROVIDER.
func (r *Repository) UpdateJobAfterUpload(ctx context.Context, id string, providerFileID string) error {
	query := fmt.Sprintf(
		`UPDATE %s.ingestion_job SET ai_provider_file_id = $1, status = 'UPLOADED_TO_PROVIDER' WHERE id = $2`,
		r.schema,
	)

	_, err := r.pool.Exec(ctx, query, providerFileID, id)
	if err != nil {
		return fmt.Errorf("update job after upload: %w", err)
	}
	return nil
}