package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

func TestIngestHandler_HappyPath(t *testing.T) {
	var enqueued bool

	handler := NewIngestHandler(
		&mockProcessor{
			startIngestion: func(_ context.Context, id string) (string, error) {
				return "job-abc", nil
			},
		},
		&mockEnqueuer{
			enqueueFn: func(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
				enqueued = true
				return nil, nil
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/ingest/:id")
	c.SetParamNames("id")
	c.SetParamValues("vid-123")

	err := handler.Ingest(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)

	if body["job_id"] != "job-abc" {
		t.Fatalf("expected job_id 'job-abc', got '%s'", body["job_id"])
	}

	if !enqueued {
		t.Fatal("expected task to be enqueued")
	}
}

func TestIngestHandler_SadPath_MissingID(t *testing.T) {
	handler := NewIngestHandler(&mockProcessor{}, &mockEnqueuer{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/ingest/:id")
	c.SetParamNames("id")
	c.SetParamValues("")

	err := handler.Ingest(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestIngestHandler_SadPath_ProcessorFails(t *testing.T) {
	handler := NewIngestHandler(
		&mockProcessor{
			startIngestion: func(_ context.Context, _ string) (string, error) {
				return "", errors.New("file not found")
			},
		},
		&mockEnqueuer{},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/ingest/:id")
	c.SetParamNames("id")
	c.SetParamValues("vid-123")

	err := handler.Ingest(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)

	if body["error"] != "file not found" {
		t.Fatalf("expected error 'file not found', got '%s'", body["error"])
	}
}

func TestIngestHandler_SadPath_EnqueueFails(t *testing.T) {
	handler := NewIngestHandler(
		&mockProcessor{
			startIngestion: func(_ context.Context, _ string) (string, error) {
				return "job-abc", nil
			},
		},
		&mockEnqueuer{
			enqueueFn: func(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
				return nil, errors.New("redis down")
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/ingest/:id")
	c.SetParamNames("id")
	c.SetParamValues("vid-123")

	err := handler.Ingest(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
