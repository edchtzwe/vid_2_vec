package handlers

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type EmbedHandler struct {
	embedder EmbedderInterface
}

func NewEmbedHandler(embedder EmbedderInterface) *EmbedHandler {
	return &EmbedHandler{embedder: embedder}
}

// Reembed triggers re-embedding for a COMPLETED job, bypassing the full
// ingestion pipeline. It reads the existing manifest from disk and
// selectively re-embeds scenes whose content has changed.
func (h *EmbedHandler) Reembed(c echo.Context) error {
	jobID := c.Param("id")
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing job id parameter",
		})
	}

	ctx := c.Request().Context()

	if err := h.embedder.ProcessJob(ctx, jobID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error":  "job not found",
				"job_id": jobID,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":  err.Error(),
			"job_id": jobID,
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "re-embedding completed",
		"job_id":  jobID,
	})
}
