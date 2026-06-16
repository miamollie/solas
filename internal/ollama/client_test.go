package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsReachable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.IsReachable(context.Background()); err != nil {
		t.Fatalf("expected reachable, got %v", err)
	}
}

func TestListModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(TagsResponse{Models: []TagModel{{Name: "llama3.1:8b"}}})
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(models) != 1 || models[0].Name != "llama3.1:8b" {
		t.Fatalf("unexpected models: %+v", models)
	}
}
