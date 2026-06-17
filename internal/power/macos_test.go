package power

import (
	"math"
	"testing"
)

func TestParsePowerOutputWatts(t *testing.T) {
	out := `
*** Sampled system activity ***
CPU Power: 5.42 W
GPU Power: 2.10 W
`

	cpu, gpu, err := parsePowerOutput(out)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !almostEqual(cpu, 5.42) {
		t.Fatalf("expected cpu=5.42, got %v", cpu)
	}
	if !almostEqual(gpu, 2.10) {
		t.Fatalf("expected gpu=2.10, got %v", gpu)
	}
}

func TestParsePowerOutputMilliwatts(t *testing.T) {
	out := `
CPU Power: 4300 mW
GPU Power: 1200 mW
`

	cpu, gpu, err := parsePowerOutput(out)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !almostEqual(cpu, 4.3) {
		t.Fatalf("expected cpu=4.3, got %v", cpu)
	}
	if !almostEqual(gpu, 1.2) {
		t.Fatalf("expected gpu=1.2, got %v", gpu)
	}
}

func TestParsePowerOutputMissingGPU(t *testing.T) {
	out := `CPU Power: 3.00 W`

	_, _, err := parsePowerOutput(out)
	if err == nil {
		t.Fatalf("expected error when gpu is missing")
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
