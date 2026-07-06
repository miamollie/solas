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

func TestNewProfilerWithModeDefaultsToDevice(t *testing.T) {
	p := NewProfilerWithMode(nil, metrics.New(), nil, 5*time.Second, nil, "invalid", nil)
	if p.mode != ProcessModeDevice {
		t.Fatalf("expected invalid mode to default to device, got %q", p.mode)
	}
}

func TestParseDockerStatsJSON(t *testing.T) {
	out := `{"CPUPerc":"2.91%","MemUsage":"46.79MiB / 7.75GiB"}`
	cpu, rss, err := parseDockerStatsJSON(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cpu != 2.91 {
		t.Fatalf("expected cpu 2.91, got %v", cpu)
	}
	if rss <= 0 {
		t.Fatalf("expected positive rss bytes, got %v", rss)
	}
}

func TestParseDockerStatsJSONInvalid(t *testing.T) {
	_, _, err := parseDockerStatsJSON("not-json")
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseByteSize(t *testing.T) {
	v, err := parseByteSize("1.5GiB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1.5*1024*1024*1024 {
		t.Fatalf("unexpected byte size conversion %v", v)
	}
}

func TestInferContainerFromBaseURL(t *testing.T) {
	if got := inferContainerFromBaseURL("http://ollama:11434"); got != "ollama" {
		t.Fatalf("expected inferred container name ollama, got %q", got)
	}
	if got := inferContainerFromBaseURL("http://127.0.0.1:11434"); got != "" {
		t.Fatalf("expected empty inferred container for loopback, got %q", got)
	}
}
