package power

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/miamollie/solas/internal/metrics"
)

// Profiler periodically samples host power and local upstream process stats.
type Profiler struct {
	collector           Collector
	metrics             *metrics.Metrics
	logger              *slog.Logger
	interval            time.Duration
	providers           map[string]string
	pidCache            map[string]int
	lastProcessSampleAt map[string]time.Time
}

// NewProfiler creates a background power profiler.
func NewProfiler(collector Collector, met *metrics.Metrics, logger *slog.Logger, interval time.Duration, providers map[string]string) *Profiler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if providers == nil {
		providers = map[string]string{}
	}
	return &Profiler{
		collector:           collector,
		metrics:             met,
		logger:              logger,
		interval:            interval,
		providers:           providers,
		pidCache:            map[string]int{},
		lastProcessSampleAt: map[string]time.Time{},
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

	now := time.Now()
	for provider, baseURL := range p.providers {
		if !p.shouldSampleProcess(provider, now) {
			continue
		}

		pid := p.pidCache[provider]
		if pid <= 0 {
			pidCtx, cancelPID := context.WithTimeout(ctx, 1500*time.Millisecond)
			resolvedPID, pidErr := resolveListeningPID(pidCtx, baseURL)
			cancelPID()
			if pidErr != nil || resolvedPID <= 0 {
				p.pidCache[provider] = 0
				p.lastProcessSampleAt[provider] = now
				p.metrics.SetLLMProcessMetrics(provider, 0, 0, 0)
				continue
			}
			pid = resolvedPID
			p.pidCache[provider] = pid
		}

		statsCtx, cancelStats := context.WithTimeout(ctx, 1500*time.Millisecond)
		cpuPercent, rssBytes, statsErr := sampleProcessStats(statsCtx, pid)
		cancelStats()
		p.lastProcessSampleAt[provider] = now
		if statsErr != nil {
			p.pidCache[provider] = 0
			p.metrics.SetLLMProcessMetrics(provider, pid, 0, 0)
			continue
		}
		p.metrics.SetLLMProcessMetrics(provider, pid, cpuPercent, rssBytes)
	}
}

func (p *Profiler) shouldSampleProcess(provider string, now time.Time) bool {
	if p.metrics.InFlight(provider) > 0 {
		return true
	}
	last := p.lastProcessSampleAt[provider]
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
