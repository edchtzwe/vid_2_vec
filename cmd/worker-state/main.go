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

	checker := aiworkers.NewStateChecker(res.Repo, res.GeminiClient, res.AsynqClient, res.StateCheckerLog)

	srv, err := res.NewAsynqServer(tasks.QueueState, 5)
	if err != nil {
		log.Fatalf("asynq server: %v", err)
	}

	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeCheckState, checker)

	go func() {
		log.Printf("worker-state listening on queue %q", tasks.QueueState)
		if err := srv.Run(mux); err != nil {
			log.Printf("asynq server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.Stop()
	log.Println("worker-state stopped")
}
