package handlers

import (
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"

	"ingestion/internal/service"
	"ingestion/internal/tasks"
)

type IngestHandler struct {
	processor   *service.Processor
	asynqClient *asynq.Client
}

func NewIngestHandler(processor *service.Processor, asynqClient *asynq.Client) *IngestHandler {
	return &IngestHandler{
		processor:   processor,
		asynqClient: asynqClient,
	}
}

func (h *IngestHandler) Ingest(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing id parameter",
		})
	}

	ctx := c.Request().Context()

	jobID, err := h.processor.StartIngestion(ctx, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
			"id":    id,
		})
	}

	task, err := tasks.NewUploadVideoTask(jobID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create upload task: " + err.Error(),
			"id":    id,
		})
	}
	if _, err := h.asynqClient.Enqueue(task); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to enqueue upload task: " + err.Error(),
			"id":    id,
		})
	}

	return c.JSON(http.StatusAccepted, map[string]string{
		"status":  "pending",
		"message": "Ingestion job created, upload will proceed in background",
		"job_id":  jobID,
		"id":      id,
	})
}
