package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"ingestion/internal/bootstrap"
	"ingestion/internal/tasks"
	aiworkers "ingestion/internal/workers/ai"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := bootstrap.Init(ctx)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer res.Close()

	analyzer := aiworkers.NewAnalyzer(res.Repo, res.GeminiClient, res.AsynqClient, res.Cfg.AnalyzerOutputDir, res.AnalyzerLog)

	srv, err := res.NewAsynqServer(tasks.QueueAnalyze, 2)
	if err != nil {
		log.Fatalf("asynq server: %v", err)
	}

	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeAnalyzeVideo, analyzer)

	go func() {
		log.Printf("worker-analyzer listening on queue %q", tasks.QueueAnalyze)
		if err := srv.Run(mux); err != nil {
			log.Printf("asynq server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.Stop()
	log.Println("worker-analyzer stopped")
}
