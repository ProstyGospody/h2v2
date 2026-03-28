package services

import "testing"

func TestParseCPUSampleLine(t *testing.T) {
	sample, err := parseCPUSampleLine("cpu  120 8 24 500 20 0 0 0 0 0")
	if err != nil {
		t.Fatalf("parseCPUSampleLine returned error: %v", err)
	}

	if sample.total != 672 {
		t.Fatalf("unexpected total: %d", sample.total)
	}
	if sample.idle != 520 {
		t.Fatalf("unexpected idle: %d", sample.idle)
	}
}

func TestParseCPUSampleLineRejectsInvalidInput(t *testing.T) {
	_, err := parseCPUSampleLine("cpu")
	if err == nil {
		t.Fatal("expected error for invalid cpu line")
	}
}

func TestCalculateCPUUsage(t *testing.T) {
	usage, err := calculateCPUUsage(
		cpuSample{total: 1000, idle: 600},
		cpuSample{total: 1300, idle: 750},
	)
	if err != nil {
		t.Fatalf("calculateCPUUsage returned error: %v", err)
	}

	if usage != 50 {
		t.Fatalf("unexpected usage: %.2f", usage)
	}
}

func TestCalculateCPUUsageRejectsNonIncreasingTotal(t *testing.T) {
	_, err := calculateCPUUsage(
		cpuSample{total: 1000, idle: 600},
		cpuSample{total: 1000, idle: 650},
	)
	if err == nil {
		t.Fatal("expected error when total does not increase")
	}
}
