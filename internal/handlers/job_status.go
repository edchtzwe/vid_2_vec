package handlers

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"ingestion/internal/repository"
)

type JobHandler struct {
	repo repository.RepositoryInterface
}

func NewJobHandler(repo repository.RepositoryInterface) *JobHandler {
	return &JobHandler{repo: repo}
}

func (h *JobHandler) GetStatus(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing id parameter",
		})
	}

	job, err := h.repo.GetJobByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "job not found",
				"id":    id,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, job)
}
