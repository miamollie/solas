package power

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/miamollie/solas/internal/metrics"
)

const (
	ProcessModeDevice    = "device"
	ProcessModeContainer = "container"
	profiledLLMLabel     = "ollama"
)

var sizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]+)$`)

type commandOutputFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

// Profiler periodically samples host power and local upstream process stats.
type Profiler struct {
	collector           Collector
	metrics             *metrics.Metrics
	logger              *slog.Logger
	interval            time.Duration
	mode                string
	baseURL             string
	containerName       string
	pid                 int
	lastProcessSampleAt time.Time
	commandOutput       commandOutputFunc
}

// NewProfiler creates a background power profiler.
func NewProfiler(collector Collector, met *metrics.Metrics, logger *slog.Logger, interval time.Duration, baseURL string) *Profiler {
	return NewProfilerWithMode(collector, met, logger, interval, baseURL, ProcessModeDevice, "")
}

// NewProfilerWithMode creates a background power profiler with process sampling mode.
func NewProfilerWithMode(collector Collector, met *metrics.Metrics, logger *slog.Logger, interval time.Duration, baseURL, mode, containerName string) *Profiler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if mode != ProcessModeContainer {
		mode = ProcessModeDevice
	}
	return &Profiler{
		collector:     collector,
		metrics:       met,
		logger:        logger,
		interval:      interval,
		mode:          mode,
		baseURL:       baseURL,
		containerName: strings.TrimSpace(containerName),
		commandOutput: runCommandOutput,
	}
}

// Start runs the profiling loop until context cancellation.
func (p *Profiler) Start(ctx context.Context) {
	if p.collector == nil || p.metrics == nil {
		return
	}
	p.sampleOnce(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.sampleOnce(ctx)
		}
	}
}

func (p *Profiler) sampleOnce(ctx context.Context) {
	health := p.collector.Health()
	p.metrics.SetPowerCollectorHealthy(health.Available)

	collectCtx, cancelCollect := context.WithTimeout(ctx, p.interval)
	sample, err := p.collector.Collect(collectCtx)
	cancelCollect()
	if err != nil {
		p.metrics.SetPowerCollectorHealthy(false)
		if p.logger != nil {
			p.logger.Debug("power sample failed", "error", err)
		}
	} else {
		p.metrics.SetPowerCollectorHealthy(true)
		p.metrics.SetPowerSample(sample.CPUWatts, sample.GPUWatts, sample.TotalWatts)
	}

	if strings.TrimSpace(p.baseURL) == "" {
		return
	}

	now := time.Now()
	if !p.shouldSampleProcess(now) {
		return
	}
	p.lastProcessSampleAt = now

	if p.mode == ProcessModeContainer {
		p.sampleContainerProcess(ctx)
		return
	}

	p.sampleHostProcess(ctx)
}

func (p *Profiler) sampleHostProcess(ctx context.Context) {
	pid := p.pid
	if pid <= 0 {
		pidCtx, cancelPID := context.WithTimeout(ctx, 1500*time.Millisecond)
		resolvedPID, pidErr := resolveListeningPID(pidCtx, p.baseURL)
		cancelPID()
		if pidErr != nil || resolvedPID <= 0 {
			p.pid = 0
			p.metrics.SetLLMProcessMetrics(profiledLLMLabel, 0, 0, 0)
			return
		}
		pid = resolvedPID
		p.pid = pid
	}

	statsCtx, cancelStats := context.WithTimeout(ctx, 1500*time.Millisecond)
	cpuPercent, rssBytes, statsErr := sampleProcessStats(statsCtx, pid)
	cancelStats()
	if statsErr != nil {
		p.pid = 0
		p.metrics.SetLLMProcessMetrics(profiledLLMLabel, pid, 0, 0)
		return
	}
	p.metrics.SetLLMProcessMetrics(profiledLLMLabel, pid, cpuPercent, rssBytes)
}

func (p *Profiler) sampleContainerProcess(ctx context.Context) {
	containerName := p.containerName
	if containerName == "" {
		containerName = inferContainerFromBaseURL(p.baseURL)
	}
	if containerName == "" {
		p.metrics.SetLLMProcessMetrics(profiledLLMLabel, 0, 0, 0)
		return
	}
	statsCtx, cancelStats := context.WithTimeout(ctx, 1500*time.Millisecond)
	cpuPercent, rssBytes, statsErr := sampleContainerStats(statsCtx, containerName, p.commandOutput)
	cancelStats()
	if statsErr != nil {
		p.metrics.SetLLMProcessMetrics(profiledLLMLabel, 0, 0, 0)
		return
	}
	p.metrics.SetLLMProcessMetrics(profiledLLMLabel, 0, cpuPercent, rssBytes)
}

func (p *Profiler) shouldSampleProcess(now time.Time) bool {
	last := p.lastProcessSampleAt
	if last.IsZero() {
		return true
	}
	return now.Sub(last) >= idleProcessSamplingInterval(p.interval)
}

func idleProcessSamplingInterval(baseInterval time.Duration) time.Duration {
	if baseInterval <= 0 {
		baseInterval = 5 * time.Second
	}
	return baseInterval * 6
}

func resolveListeningPID(ctx context.Context, baseURL string) (int, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("parse base url: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return 0, errors.New("base url missing host")
	}
	if !isLocalHost(host) {
		return 0, errors.New("base url is not local")
	}

	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}

	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-Fp")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("resolve listening pid: %w", err)
	}
	pid, err := parseListeningPIDOutput(string(out))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func sampleProcessStats(ctx context.Context, pid int) (float64, float64, error) {
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "pcpu=", "-o", "rss=")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("sample process stats: %w", err)
	}
	cpuPercent, rssBytes, err := parsePSStatsOutput(string(out))
	if err != nil {
		return 0, 0, err
	}
	return cpuPercent, rssBytes, nil
}

func parseListeningPIDOutput(out string) (int, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 2 || line[0] != 'p' {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err == nil && pid > 0 {
			return pid, nil
		}
	}
	return 0, errors.New("no listening pid found")
}

func parsePSStatsOutput(out string) (float64, float64, error) {
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return 0, 0, errors.New("missing process stat fields")
	}
	cpuPercent, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse process cpu: %w", err)
	}
	rssKB, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse process rss: %w", err)
	}
	return cpuPercent, rssKB * 1024, nil
}

func sampleContainerStats(ctx context.Context, containerName string, run commandOutputFunc) (float64, float64, error) {
	if strings.TrimSpace(containerName) == "" {
		return 0, 0, errors.New("container name is required")
	}
	if run == nil {
		run = runCommandOutput
	}
	out, err := run(ctx, "docker", "stats", "--no-stream", "--format", "json", containerName)
	if err != nil {
		return 0, 0, fmt.Errorf("sample container stats: %w", err)
	}
	return parseDockerStatsJSON(strings.TrimSpace(string(out)))
}

func parseDockerStatsJSON(out string) (float64, float64, error) {
	if out == "" {
		return 0, 0, errors.New("empty docker stats output")
	}
	lines := strings.Split(out, "\n")
	line := strings.TrimSpace(lines[0])
	if line == "" {
		return 0, 0, errors.New("empty docker stats output")
	}

	var payload struct {
		CPUPerc  string
		MemUsage string
	}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return 0, 0, fmt.Errorf("parse docker stats json: %w", err)
	}
	cpuPercent, err := parsePercent(payload.CPUPerc)
	if err != nil {
		return 0, 0, err
	}
	rssBytes, err := parseMemUsage(payload.MemUsage)
	if err != nil {
		return 0, 0, err
	}
	return cpuPercent, rssBytes, nil
}

func parsePercent(v string) (float64, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "%"))
	if trimmed == "" {
		return 0, errors.New("missing percent value")
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("parse percent: %w", err)
	}
	return parsed, nil
}

func parseMemUsage(v string) (float64, error) {
	parts := strings.Split(v, "/")
	if len(parts) == 0 {
		return 0, errors.New("missing memory usage")
	}
	return parseByteSize(parts[0])
}

func parseByteSize(v string) (float64, error) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return 0, errors.New("missing byte size")
	}
	matches := sizePattern.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return 0, fmt.Errorf("unrecognized byte size %q", v)
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("parse size value: %w", err)
	}
	unit := strings.ToLower(matches[2])
	multipliers := map[string]float64{
		"b":   1,
		"kb":  1000,
		"mb":  1000 * 1000,
		"gb":  1000 * 1000 * 1000,
		"tb":  1000 * 1000 * 1000 * 1000,
		"kib": 1024,
		"mib": 1024 * 1024,
		"gib": 1024 * 1024 * 1024,
		"tib": 1024 * 1024 * 1024 * 1024,
	}
	multiplier, ok := multipliers[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported byte size unit %q", unit)
	}
	return value * multiplier, nil
}

func inferContainerFromBaseURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" || strings.EqualFold(host, "localhost") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return ""
		}
		return ""
	}
	return host
}

func runCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

func isLocalHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
