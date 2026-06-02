package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// Queue names — each worker process listens on exactly one queue.
const (
	QueueUpload  = "upload"
	QueueState   = "state"
	QueueAnalyze = "analyze"
	QueueEmbed   = "embed"
)

// Task type identifiers.
const (
	TypeUploadVideo  = "video:upload"
	TypeCheckState   = "video:check_state"
	TypeAnalyzeVideo = "video:analyze"
	TypeEmbedVideo   = "video:embed"
)

// JobPayload is the standard payload carried by every task in the pipeline.
type JobPayload struct {
	JobID string `json:"job_id"`
}

func NewUploadVideoTask(jobID string, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, fmt.Errorf("marshal upload payload: %w", err)
	}
	defaults := []asynq.Option{asynq.Queue(QueueUpload), asynq.MaxRetry(3)}
	return asynq.NewTask(TypeUploadVideo, payload, append(defaults, opts...)...), nil
}

func NewCheckStateTask(jobID string, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, fmt.Errorf("marshal state payload: %w", err)
	}
	defaults := []asynq.Option{asynq.Queue(QueueState), asynq.MaxRetry(20)}
	return asynq.NewTask(TypeCheckState, payload, append(defaults, opts...)...), nil
}

func NewAnalyzeVideoTask(jobID string, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, fmt.Errorf("marshal analyze payload: %w", err)
	}
	defaults := []asynq.Option{asynq.Queue(QueueAnalyze), asynq.MaxRetry(3)}
	return asynq.NewTask(TypeAnalyzeVideo, payload, append(defaults, opts...)...), nil
}

func NewEmbedVideoTask(jobID string, opts ...asynq.Option) (*asynq.Task, error) {
	payload, err := json.Marshal(JobPayload{JobID: jobID})
	if err != nil {
		return nil, fmt.Errorf("marshal embed payload: %w", err)
	}
	defaults := []asynq.Option{asynq.Queue(QueueEmbed), asynq.MaxRetry(3)}
	return asynq.NewTask(TypeEmbedVideo, payload, append(defaults, opts...)...), nil
}
