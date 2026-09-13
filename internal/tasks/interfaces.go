package tasks

import "github.com/hibiken/asynq"

type AsynqEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
