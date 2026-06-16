package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/miamollie/greenops-local-llm/internal/ollama"
)

type fakeReadyChecker struct {
	err    error
	models []ollama.TagModel
}

func (f fakeReadyChecker) IsReachable(_ context.Context) error {
	return f.err
}

func (f fakeReadyChecker) ListModels(_ context.Context) ([]ollama.TagModel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.models, nil
}

func TestHealthEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", payload["status"])
	}
}

func TestReadyEndpointHealthy(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReadyEndpointUnavailable(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{err: errors.New("down")})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestModelsEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{models: []ollama.TagModel{{Name: "qwen3:32b"}}})
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Object != "list" || len(payload.Data) != 1 || payload.Data[0].ID != "qwen3:32b" {
		t.Fatalf("unexpected payload: %s", rr.Body.String())
	}
}
