package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"ingestion/internal/repository"
)

func TestJobHandler_HappyPath(t *testing.T) {
	now := time.Now()

	handler := NewJobHandler(
		&mockRepo{
			getJobByID: func(_ context.Context, id string) (*repository.IngestionJob, error) {
				return &repository.IngestionJob{
					ID:            id,
					SourceVideoID: "vid-1",
					Status:        "COMPLETED",
					AIProviderName: "google",
					AIModelName:   "gemini-2.5-flash",
					CreatedAt:     now,
					UpdatedAt:     now,
				}, nil
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/job-info/:id")
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body repository.IngestionJob
	json.Unmarshal(rec.Body.Bytes(), &body)

	if body.ID != "job-1" {
		t.Fatalf("expected ID 'job-1', got '%s'", body.ID)
	}

	if body.Status != "COMPLETED" {
		t.Fatalf("expected status 'COMPLETED', got '%s'", body.Status)
	}
}

func TestJobHandler_SadPath_MissingID(t *testing.T) {
	handler := NewJobHandler(&mockRepo{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/job-info/:id")
	c.SetParamNames("id")
	c.SetParamValues("")

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestJobHandler_SadPath_NotFound(t *testing.T) {
	handler := NewJobHandler(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return nil, pgx.ErrNoRows
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/job-info/:id")
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestJobHandler_SadPath_DatabaseError(t *testing.T) {
	handler := NewJobHandler(
		&mockRepo{
			getJobByID: func(_ context.Context, _ string) (*repository.IngestionJob, error) {
				return nil, errors.New("connection refused")
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/job-info/:id")
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
