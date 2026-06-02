package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Data Structures ─────────────────────────────────────────────────────────

type VideoScene struct {
	ID            string    `db:"id" json:"id"`
	SourceVideoID string    `db:"source_video_id" json:"source_video_id"`
	StartTime     float64   `db:"start_time" json:"start_time"`
	EndTime       float64   `db:"end_time" json:"end_time"`
	Content       string    `db:"content" json:"content"`
	Name          string    `db:"name" json:"name"`
	ContentHash   string    `db:"content_hash" json:"content_hash,omitempty"`
	Embedding[]float64 `db:"embedding" json:"embedding,omitempty"`
	AIProvider    string    `db:"ai_provider" json:"ai_provider"`
	AIModel       string    `db:"ai_model" json:"ai_model"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type CreateVideoSceneParams struct {
	SourceVideoID string
	StartTime     float64
	EndTime       float64
	Content       string
	Name          string
	ContentHash   string
	Embedding[]float64
	AIProvider    string
	AIModel       string
}

type UpdateVideoSceneParams struct {
	ID          string
	Content     string
	Name        string
	ContentHash string
	Embedding[]float64
	AIProvider  string
	AIModel     string
}

// ─── Methods ─────────────────────────────────────────────────────────────────

// CreateVideoScene inserts a new scene analysis record and returns the new row's ID.
// The embedding column is pgvector's vector type, which accepts the text
// representation "[0.1,0.2,...]" — not raw bytes — so we marshal to a string.
func (r *Repository) CreateVideoScene(ctx context.Context, params CreateVideoSceneParams) (string, error) {
	embeddingStr := marshalEmbedding(params.Embedding)
	if embeddingStr == "" {
		return "", fmt.Errorf("failed to marshal embedding")
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.video_scenes
		(source_video_id, start_time, end_time, content, name, content_hash, embedding, ai_provider, ai_model)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9)
		RETURNING id
	`, r.schema)

	var id string
	err := r.pool.QueryRow(ctx, query,
		params.SourceVideoID,
		params.StartTime,
		params.EndTime,
		params.Content,
		params.Name,
		params.ContentHash,
		embeddingStr,
		params.AIProvider,
		params.AIModel,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create video scene: %w", err)
	}

	return id, nil
}

// UpdateVideoScene updates an existing scene's content, embedding, and hash.
func (r *Repository) UpdateVideoScene(ctx context.Context, params UpdateVideoSceneParams) error {
	embeddingStr := marshalEmbedding(params.Embedding)
	if embeddingStr == "" {
		return fmt.Errorf("failed to marshal embedding")
	}

	query := fmt.Sprintf(`
		UPDATE %s.video_scenes
		SET content = $1, name = $2, content_hash = $3, embedding = $4::vector,
		    ai_provider = $5, ai_model = $6, updated_at = NOW()
		WHERE id = $7
	`, r.schema)

	_, err := r.pool.Exec(ctx, query,
		params.Content,
		params.Name,
		params.ContentHash,
		embeddingStr,
		params.AIProvider,
		params.AIModel,
		params.ID,
	)
	if err != nil {
		return fmt.Errorf("update video scene: %w", err)
	}
	return nil
}

// DeleteScenesBySourceVideoID removes all scenes for a given video.
func (r *Repository) DeleteScenesBySourceVideoID(ctx context.Context, sourceVideoID string) error {
	query := fmt.Sprintf(`DELETE FROM %s.video_scenes WHERE source_video_id = $1`, r.schema)
	_, err := r.pool.Exec(ctx, query, sourceVideoID)
	if err != nil {
		return fmt.Errorf("delete scenes for video %s: %w", sourceVideoID, err)
	}
	return nil
}

// DeleteScenesNotInIDs removes scenes for a video whose IDs are not in the provided keep list.
func (r *Repository) DeleteScenesNotInIDs(ctx context.Context, sourceVideoID string, keepIDs[]string) error {
	if len(keepIDs) == 0 {
		return r.DeleteScenesBySourceVideoID(ctx, sourceVideoID)
	}

	query := fmt.Sprintf(`
		DELETE FROM %s.video_scenes
		WHERE source_video_id = $1 AND id != ALL($2::uuid[])
	`, r.schema)

	_, err := r.pool.Exec(ctx, query, sourceVideoID, keepIDs)
	if err != nil {
		return fmt.Errorf("delete stale scenes for video %s: %w", sourceVideoID, err)
	}
	return nil
}

func marshalEmbedding(embedding []float64) string {
	if len(embedding) == 0 {
		return "[]"
	}
	b, err := json.Marshal(embedding)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetScenesByVideoID retrieves all scenes for a specific video, ordered by start time.
func (r *Repository) GetScenesByVideoID(ctx context.Context, videoID string) ([]VideoScene, error) {
	query := fmt.Sprintf(`
		SELECT id, source_video_id, start_time, end_time, content, name,
		       COALESCE(content_hash, '') AS content_hash,
		       embedding::text, ai_provider, ai_model, created_at
		FROM %s.video_scenes
		WHERE source_video_id = $1
		ORDER BY start_time ASC
	`, r.schema)

	rows, err := r.pool.Query(ctx, query, videoID)
	if err != nil {
		return nil, fmt.Errorf("query scenes: %w", err)
	}
	defer rows.Close()

	var scenes[]VideoScene
	for rows.Next() {
		var s VideoScene
		var embeddingStr string

		err := rows.Scan(
			&s.ID,
			&s.SourceVideoID,
			&s.StartTime,
			&s.EndTime,
			&s.Content,
			&s.Name,
			&s.ContentHash,
			&embeddingStr,
			&s.AIProvider,
			&s.AIModel,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan scene: %w", err)
		}

		if embeddingStr != "" {
			if err := json.Unmarshal([]byte(embeddingStr), &s.Embedding); err != nil {
				return nil, fmt.Errorf("unmarshal embedding: %w", err)
			}
		}

		scenes = append(scenes, s)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return scenes, nil
}

// CHANGE: Added BulkReplaceScenes to handle the deletion and insertion loop in a single transaction.
func (r *Repository) BulkReplaceScenes(ctx context.Context, sourceVideoID string, scenes[]CreateVideoSceneParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	delQuery := fmt.Sprintf(`DELETE FROM %s.video_scenes WHERE source_video_id = $1`, r.schema)
	if _, err := tx.Exec(ctx, delQuery, sourceVideoID); err != nil {
		return fmt.Errorf("delete existing scenes: %w", err)
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.video_scenes
		(source_video_id, start_time, end_time, content, name, content_hash, embedding, ai_provider, ai_model)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9)
	`, r.schema)

	for _, p := range scenes {
		embStr := marshalEmbedding(p.Embedding)
		if _, err := tx.Exec(ctx, insertQuery, p.SourceVideoID, p.StartTime, p.EndTime, p.Content, p.Name, p.ContentHash, embStr, p.AIProvider, p.AIModel); err != nil {
			return fmt.Errorf("insert scene: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// CHANGE: Added SyncScenes to handle creates, updates, and deletes in a single transaction.
func (r *Repository) SyncScenes(ctx context.Context, sourceVideoID string, toCreate []CreateVideoSceneParams, toUpdate []UpdateVideoSceneParams, toDeleteIDs[]string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if len(toDeleteIDs) > 0 {
		delQuery := fmt.Sprintf(`DELETE FROM %s.video_scenes WHERE source_video_id = $1 AND id = ANY($2::uuid[])`, r.schema)
		if _, err := tx.Exec(ctx, delQuery, sourceVideoID, toDeleteIDs); err != nil {
			return fmt.Errorf("delete scenes: %w", err)
		}
	}

	updateQuery := fmt.Sprintf(`
		UPDATE %s.video_scenes
		SET content = $1, name = $2, content_hash = $3, embedding = $4::vector,
		    ai_provider = $5, ai_model = $6, updated_at = NOW()
		WHERE id = $7
	`, r.schema)
	for _, p := range toUpdate {
		embStr := marshalEmbedding(p.Embedding)
		if _, err := tx.Exec(ctx, updateQuery, p.Content, p.Name, p.ContentHash, embStr, p.AIProvider, p.AIModel, p.ID); err != nil {
			return fmt.Errorf("update scene %s: %w", p.ID, err)
		}
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.video_scenes
		(source_video_id, start_time, end_time, content, name, content_hash, embedding, ai_provider, ai_model)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9)
	`, r.schema)
	for _, p := range toCreate {
		embStr := marshalEmbedding(p.Embedding)
		if _, err := tx.Exec(ctx, insertQuery, p.SourceVideoID, p.StartTime, p.EndTime, p.Content, p.Name, p.ContentHash, embStr, p.AIProvider, p.AIModel); err != nil {
			return fmt.Errorf("insert scene: %w", err)
		}
	}

	return tx.Commit(ctx)
}