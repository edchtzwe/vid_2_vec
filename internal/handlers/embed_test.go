package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

func TestEmbedHandler_HappyPath(t *testing.T) {
	handler := NewEmbedHandler(
		&mockEmbedder{
			processJob: func(_ context.Context, _ string) error {
				return nil
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/manual-embed/:id")
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	err := handler.Reembed(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)

	if body["status"] != "ok" {
		t.Fatalf("expected status 'ok', got '%s'", body["status"])
	}
}

func TestEmbedHandler_SadPath_MissingID(t *testing.T) {
	handler := NewEmbedHandler(&mockEmbedder{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/manual-embed/:id")
	c.SetParamNames("id")
	c.SetParamValues("")

	err := handler.Reembed(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestEmbedHandler_SadPath_NotFound(t *testing.T) {
	handler := NewEmbedHandler(
		&mockEmbedder{
			processJob: func(_ context.Context, _ string) error {
				return fmt.Errorf("get job nonexistent: %w", pgx.ErrNoRows)
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/manual-embed/:id")
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := handler.Reembed(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestEmbedHandler_SadPath_EmbedFails(t *testing.T) {
	handler := NewEmbedHandler(
		&mockEmbedder{
			processJob: func(_ context.Context, _ string) error {
				return errors.New("embedding API down")
			},
		},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/manual-embed/:id")
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	err := handler.Reembed(c)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
