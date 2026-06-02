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

	embedder := aiworkers.NewEmbedder(res.Repo, res.GeminiClient, res.Cfg.AnalyzerOutputDir, res.EmbedderLog)

	srv, err := res.NewAsynqServer(tasks.QueueEmbed, 2)
	if err != nil {
		log.Fatalf("asynq server: %v", err)
	}

	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeEmbedVideo, embedder)

	go func() {
		log.Printf("worker-embedder listening on queue %q", tasks.QueueEmbed)
		if err := srv.Run(mux); err != nil {
			log.Printf("asynq server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.Stop()
	log.Println("worker-embedder stopped")
}
