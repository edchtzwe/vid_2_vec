package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"ingestion/internal/bootstrap"
	"ingestion/internal/handlers"
	"ingestion/internal/routes"
	"ingestion/internal/service"
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

	processor := service.NewProcessor(res.Repo, res.Cfg)
	embedder := aiworkers.NewEmbedder(res.Repo, res.GeminiClient, res.Cfg.AnalyzerOutputDir, res.EmbedderLog)
	ingestHandler := handlers.NewIngestHandler(processor, res.AsynqClient)
	jobHandler := handlers.NewJobHandler(res.Repo)
	embedHandler := handlers.NewEmbedHandler(embedder)
	router := routes.NewRouter(ingestHandler, jobHandler, embedHandler)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/health-check", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	router.Register(e)

	go func() {
		log.Printf("starting API server on %s", res.Cfg.ListenAddr())
		if err := e.Start(res.Cfg.ListenAddr()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("API server stopped")
}
