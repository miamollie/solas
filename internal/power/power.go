package power

import (
	"context"
	"time"
)

// Sample captures a single machine power reading.
type Sample struct {
	Timestamp  time.Time
	CPUWatts   float64
	GPUWatts   float64
	TotalWatts float64
}

// NewSample builds a sample and calculates total power from CPU and GPU values.
func NewSample(ts time.Time, cpuWatts, gpuWatts float64) Sample {
	return Sample{
		Timestamp:  ts,
		CPUWatts:   cpuWatts,
		GPUWatts:   gpuWatts,
		TotalWatts: cpuWatts + gpuWatts,
	}
}

// Health describes collector availability and failure reason when unavailable.
type Health struct {
	Available bool
	Reason    string
}

// Collector defines power sampling capabilities for a host platform.
type Collector interface {
	Collect(ctx context.Context) (Sample, error)
	Health() Health
}
