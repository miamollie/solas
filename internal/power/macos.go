package power

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var powerLinePattern = regexp.MustCompile(`(?im)^\s*(CPU|GPU)\s+Power:\s*([0-9]+(?:\.[0-9]+)?)\s*(mW|W)\b`)

// MacOSCollector collects machine power from macOS powermetrics.
type MacOSCollector struct {
	mu     sync.RWMutex
	health Health
}

// NewMacOSCollector creates a collector backed by the powermetrics CLI.
func NewMacOSCollector() *MacOSCollector {
	c := &MacOSCollector{health: Health{Available: runtime.GOOS == "darwin"}}
	if runtime.GOOS != "darwin" {
		c.health.Reason = "unsupported platform"
	}
	return c
}

// Health reports collector availability.
func (c *MacOSCollector) Health() Health {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.health
}

// Collect runs powermetrics and returns a timestamped power sample.
func (c *MacOSCollector) Collect(ctx context.Context) (Sample, error) {
	if runtime.GOOS != "darwin" {
		err := errors.New("powermetrics collection is only supported on darwin")
		c.setHealth(false, err.Error())
		return Sample{}, err
	}

	cmd := exec.CommandContext(ctx, "powermetrics", "-n", "1", "-i", "1000", "--samplers", "cpu_power,gpu_power")
	out, err := cmd.CombinedOutput()
	if err != nil {
		wrapped := fmt.Errorf("powermetrics failed: %w", err)
		c.setHealth(false, wrapped.Error())
		return Sample{}, wrapped
	}

	cpuWatts, gpuWatts, err := parsePowerOutput(string(out))
	if err != nil {
		c.setHealth(false, err.Error())
		return Sample{}, err
	}

	sample := NewSample(time.Now(), cpuWatts, gpuWatts)
	c.setHealth(true, "")
	return sample, nil
}

func (c *MacOSCollector) setHealth(available bool, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health = Health{Available: available, Reason: reason}
}

func parsePowerOutput(output string) (float64, float64, error) {
	matches := powerLinePattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, 0, errors.New("powermetrics output missing CPU/GPU power lines")
	}

	var cpuWatts float64
	var gpuWatts float64
	seenCPU := false
	seenGPU := false

	for _, m := range matches {
		if len(m) != 4 {
			continue
		}
		component := strings.ToUpper(m[1])
		rawValue := m[2]
		unit := strings.ToLower(m[3])

		v, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parse %s power value: %w", component, err)
		}
		if unit == "mw" {
			v = v / 1000.0
		}

		switch component {
		case "CPU":
			cpuWatts = v
			seenCPU = true
		case "GPU":
			gpuWatts = v
			seenGPU = true
		}
	}

	if !seenCPU || !seenGPU {
		return 0, 0, errors.New("powermetrics output must contain both CPU and GPU power")
	}

	return cpuWatts, gpuWatts, nil
}
