package power

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubCollector struct {
	sample Sample
	err    error
	health Health
}

func (s stubCollector) Collect(_ context.Context) (Sample, error) {
	return s.sample, s.err
}

func (s stubCollector) Health() Health {
	return s.health
}

func TestNewSampleComputesTotalWatts(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	s := NewSample(ts, 12.5, 7.75)

	if !s.Timestamp.Equal(ts) {
		t.Fatalf("unexpected timestamp: %v", s.Timestamp)
	}
	if s.TotalWatts != 20.25 {
		t.Fatalf("expected total watts 20.25, got %v", s.TotalWatts)
	}
}

func TestCollectorContract(t *testing.T) {
	expectedErr := errors.New("collector unavailable")
	expectedSample := NewSample(time.Now(), 10, 5)
	c := stubCollector{
		sample: expectedSample,
		err:    expectedErr,
		health: Health{Available: false, Reason: "powermetrics missing"},
	}

	gotSample, gotErr := c.Collect(context.Background())
	if !errors.Is(gotErr, expectedErr) {
		t.Fatalf("expected collect error %v, got %v", expectedErr, gotErr)
	}
	if gotSample.TotalWatts != expectedSample.TotalWatts {
		t.Fatalf("expected sample total %v, got %v", expectedSample.TotalWatts, gotSample.TotalWatts)
	}

	h := c.Health()
	if h.Available {
		t.Fatalf("expected collector unavailable")
	}
	if h.Reason == "" {
		t.Fatalf("expected unavailable reason")
	}
}
