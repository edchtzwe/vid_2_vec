package routes

import (
	"github.com/labstack/echo/v4"

	"ingestion/internal/handlers"
)

type Router struct {
	IngestHandler *handlers.IngestHandler
	JobHandler    *handlers.JobHandler
	EmbedHandler  *handlers.EmbedHandler
}

func NewRouter(ingestHandler *handlers.IngestHandler, jobHandler *handlers.JobHandler, embedHandler *handlers.EmbedHandler) *Router {
	return &Router{
		IngestHandler: ingestHandler,
		JobHandler:    jobHandler,
		EmbedHandler:  embedHandler,
	}
}

func (r *Router) Register(e *echo.Echo) {
	e.POST("/ingest/:id", r.IngestHandler.Ingest)
	e.GET("/job-info/:id", r.JobHandler.GetStatus)
	e.POST("/manual-embed/:id", r.EmbedHandler.Reembed)
}
