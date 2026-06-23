package power

import (
	"testing"
	"time"

	"github.com/miamollie/solas/internal/metrics"
)

func TestParseListeningPIDOutput(t *testing.T) {
	pid, err := parseListeningPIDOutput("p1234\np987\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 1234 {
		t.Fatalf("expected pid 1234, got %d", pid)
	}
}

func TestParseListeningPIDOutputMissing(t *testing.T) {
	_, err := parseListeningPIDOutput("nonsense\n")
	if err == nil {
		t.Fatalf("expected parse error for missing pid")
	}
}

func TestParsePSStatsOutput(t *testing.T) {
	cpu, rss, err := parsePSStatsOutput("14.2  34567\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cpu != 14.2 {
		t.Fatalf("expected cpu 14.2, got %v", cpu)
	}
	if rss != 34567*1024 {
		t.Fatalf("expected rss bytes %v, got %v", 34567*1024, rss)
	}
}

func TestIsLocalHost(t *testing.T) {
	if !isLocalHost("localhost") || !isLocalHost("127.0.0.1") || !isLocalHost("::1") {
		t.Fatalf("expected loopback hosts to be local")
	}
	if isLocalHost("api.openai.com") {
		t.Fatalf("expected remote host to be non-local")
	}
}

func TestIdleProcessSamplingInterval(t *testing.T) {
	if got := idleProcessSamplingInterval(5 * time.Second); got != 30*time.Second {
		t.Fatalf("expected idle interval 30s, got %v", got)
	}
}

func TestShouldSampleProcessWithInflight(t *testing.T) {
	m := metrics.New()
	m.IncInFlight("ollama")
	p := NewProfiler(nil, m, nil, 5*time.Second, map[string]string{"ollama": "http://127.0.0.1:11434"})
	p.lastProcessSampleAt["ollama"] = time.Now()

	if !p.shouldSampleProcess("ollama", time.Now()) {
		t.Fatalf("expected active provider to be sampled immediately")
	}
}

func TestShouldSampleProcessIdleBackoff(t *testing.T) {
	m := metrics.New()
	p := NewProfiler(nil, m, nil, 5*time.Second, map[string]string{"ollama": "http://127.0.0.1:11434"})
	now := time.Now()
	p.lastProcessSampleAt["ollama"] = now

	if p.shouldSampleProcess("ollama", now.Add(10*time.Second)) {
		t.Fatalf("expected idle provider sampling to back off before interval")
	}
	if !p.shouldSampleProcess("ollama", now.Add(31*time.Second)) {
		t.Fatalf("expected idle provider sampling after backoff interval")
	}
}
