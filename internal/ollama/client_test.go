package ollama

import (
	"context"
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
