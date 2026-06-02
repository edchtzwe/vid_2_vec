package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Data Structures ─────────────────────────────────────────────────────────

// FileMetadata represents the file info passed into the repository.
type FileMetadata struct {
	OriginalName string
	MimeType     string
	Size         int64
	Filename     string
	Path         string
}

// SourceVideo represents a row in the source_video table.
type SourceVideo struct {
	ID           string    `db:"id" json:"id"`
	OriginalName string    `db:"original_name" json:"original_name"`
	MimeType     string    `db:"mime_type" json:"mime_type"`
	Size         int64     `db:"size" json:"size"`
	Filename     string    `db:"filename" json:"filename"`
	Path         string    `db:"path" json:"path"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	Name         *string   `db:"name" json:"name,omitempty"` // Nullable
	Duration     float64   `db:"duration" json:"duration"`
}

// ─── Repository Definition ───────────────────────────────────────────────────

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

// New creates a new instance of the Repository.
func New(pool *pgxpool.Pool, schema string) *Repository {
	return &Repository{pool: pool, schema: schema}
}

// ─── Methods ─────────────────────────────────────────────────────────────────

// GetSourceVideo retrieves video metadata to verify existence and get the file path.
func (r *Repository) GetSourceVideo(ctx context.Context, id string) (*SourceVideo, error) {
	var v SourceVideo

	// Select necessary fields.
	query := fmt.Sprintf(`
		SELECT id, original_name, path, mime_type, size 
		FROM %s.source_video 
		WHERE id = $1`, r.schema)

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&v.ID,
		&v.OriginalName,
		&v.Path,
		&v.MimeType,
		&v.Size,
	)
	if err != nil {
		return nil, fmt.Errorf("video not found in db: %w", err)
	}

	return &v, nil
}

// CreateSourceVideo inserts a new video record into the database.
func (r *Repository) CreateSourceVideo(
	ctx context.Context,
	file FileMetadata,
	id string,
	duration float64,
) (*SourceVideo, error) {
	var v SourceVideo

	query := fmt.Sprintf(`INSERT INTO %s.source_video
		 (id, original_name, mime_type, size, filename, path, duration)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`, r.schema)

	err := r.pool.QueryRow(ctx,
		query,
		id, file.OriginalName, file.MimeType, file.Size, file.Filename, file.Path, duration,
	).Scan(&v.ID, &v.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("CreateSourceVideo: %w", err)
	}

	return &v, nil
}
