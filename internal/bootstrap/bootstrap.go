package bootstrap

import (
	"context"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"ingestion/internal/config"
	"ingestion/internal/gemini"
	"ingestion/internal/repository"
	"ingestion/internal/service"
)

// Resources holds every shared dependency that the cmd/ entrypoints need.
type Resources struct {
	Cfg          *config.Config
	Pool         *pgxpool.Pool
	Repo         *repository.Repository
	AsynqClient  *asynq.Client
	GeminiClient *gemini.Client

	UploaderLog     *log.Logger
	StateCheckerLog *log.Logger
	AnalyzerLog     *log.Logger
	EmbedderLog     *log.Logger
}

// Init loads config, connects to Postgres, and initialises the Asynq client
// and Gemini client. The caller is responsible for calling Close().
func Init(ctx context.Context) (*Resources, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	asynqClient := asynq.NewClient(redisOpt)

	uploaderLog := service.NewFileLogger(cfg.UploaderLogPath)
	stateCheckerLog := service.NewFileLogger(cfg.StateCheckerLogPath)
	analyzerLog := service.NewFileLogger(cfg.AnalyzerLogPath)
	embedderLog := service.NewFileLogger(cfg.EmbedderLogPath)

	geminiClient := gemini.NewClient(cfg, analyzerLog)

	repo := repository.New(pool, cfg.DBSchema)

	return &Resources{
		Cfg:             cfg,
		Pool:            pool,
		Repo:            repo,
		AsynqClient:     asynqClient,
		GeminiClient:    geminiClient,
		UploaderLog:     uploaderLog,
		StateCheckerLog: stateCheckerLog,
		AnalyzerLog:     analyzerLog,
		EmbedderLog:     embedderLog,
	}, nil
}

// Close releases all held resources.
func (r *Resources) Close() {
	if r.AsynqClient != nil {
		r.AsynqClient.Close()
	}
	if r.Pool != nil {
		r.Pool.Close()
	}
}

// NewAsynqServer creates an asynq.Server that listens on the given queue only.
func (r *Resources) NewAsynqServer(queue string, concurrency int) (*asynq.Server, error) {
	redisOpt, err := asynq.ParseRedisURI(r.Cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{queue: 1},
		StrictPriority: true,
	})
	return srv, nil
}
