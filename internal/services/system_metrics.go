package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SystemMetrics struct {
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64     `json:"memory_used_bytes"`
	MemoryTotalBytes  int64     `json:"memory_total_bytes"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	CollectedAt       time.Time `json:"collected_at"`
}

type SystemMetricsCollector struct {
	mu               sync.Mutex
	lastCPU          float64
	hasLastCPU       bool
	lastCPUSample    cpuSample
	hasLastCPUSample bool
}

type cpuSample struct {
	total int64
	idle  int64
}

func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{}
}

func (c *SystemMetricsCollector) Snapshot(ctx context.Context) (SystemMetrics, error) {
	select {
	case <-ctx.Done():
		return SystemMetrics{}, ctx.Err()
	default:
	}

	cpuUsage, err := c.readCPUUsage(ctx)
	if err != nil {
		if fallbackCPU, ok := c.lastCPUValue(); ok {
			cpuUsage = fallbackCPU
		} else {
			return SystemMetrics{}, err
		}
	}

	totalMemory, availableMemory, err := readMemoryInfo()
	if err != nil {
		return SystemMetrics{}, err
	}

	uptime, err := readUptimeSeconds()
	if err != nil {
		return SystemMetrics{}, err
	}

	usedMemory := totalMemory - availableMemory
	if usedMemory < 0 {
		usedMemory = 0
	}

	usedPercent := 0.0
	if totalMemory > 0 {
		usedPercent = (float64(usedMemory) * 100) / float64(totalMemory)
	}

	return SystemMetrics{
		CPUUsagePercent:   cpuUsage,
		MemoryUsedBytes:   usedMemory,
		MemoryTotalBytes:  totalMemory,
		MemoryUsedPercent: usedPercent,
		UptimeSeconds:     uptime,
		CollectedAt:       time.Now().UTC(),
	}, nil
}

func (c *SystemMetricsCollector) lastCPUValue() (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastCPU, c.hasLastCPU
}

func (c *SystemMetricsCollector) setLastCPUValue(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastCPU = value
	c.hasLastCPU = true
}

func (c *SystemMetricsCollector) readCPUUsage(ctx context.Context) (float64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	current, err := readCPUSample()
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	prev := c.lastCPUSample
	hasPrev := c.hasLastCPUSample
	lastCPU := c.lastCPU
	hasLast := c.hasLastCPU

	if hasPrev && current.total <= prev.total {
		c.mu.Unlock()
		if hasLast {
			return lastCPU, nil
		}
		return 0, nil
	}

	c.lastCPUSample = current
	c.hasLastCPUSample = true
	c.mu.Unlock()

	if !hasPrev {
		if hasLast {
			return lastCPU, nil
		}
		return 0, nil
	}

	usage, err := calculateCPUUsage(prev, current)
	if err != nil {
		return 0, err
	}
	c.setLastCPUValue(usage)
	return usage, nil
}

func readCPUSample() (cpuSample, error) {
	content, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, fmt.Errorf("read /proc/stat: %w", err)
	}

	firstLine := strings.SplitN(string(content), "\n", 2)[0]
	sample, err := parseCPUSampleLine(firstLine)
	if err != nil {
		return cpuSample{}, err
	}
	return sample, nil
}

func parseCPUSampleLine(line string) (cpuSample, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, fmt.Errorf("invalid cpu line")
	}

	var total int64
	var idle int64

	for idx, raw := range fields[1:] {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return cpuSample{}, fmt.Errorf("parse cpu field: %w", err)
		}
		total += value
		if idx == 3 || idx == 4 {
			idle += value
		}
	}

	return cpuSample{total: total, idle: idle}, nil
}

func calculateCPUUsage(previous cpuSample, current cpuSample) (float64, error) {
	deltaTotal := current.total - previous.total
	if deltaTotal <= 0 {
		return 0, fmt.Errorf("invalid cpu sample delta")
	}

	deltaIdle := current.idle - previous.idle
	if deltaIdle < 0 {
		deltaIdle = 0
	}
	if deltaIdle > deltaTotal {
		deltaIdle = deltaTotal
	}

	busy := deltaTotal - deltaIdle
	usage := (float64(busy) * 100) / float64(deltaTotal)
	if usage < 0 {
		return 0, nil
	}
	if usage > 100 {
		return 100, nil
	}
	return usage, nil
}

func readMemoryInfo() (totalBytes int64, availableBytes int64, err error) {
	content, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}

	var totalKB int64
	var availableKB int64
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}

		switch key {
		case "MemTotal":
			totalKB = value
		case "MemAvailable":
			availableKB = value
		}
	}

	if totalKB <= 0 {
		return 0, 0, fmt.Errorf("memtotal not found in /proc/meminfo")
	}
	if availableKB < 0 {
		availableKB = 0
	}
	if availableKB > totalKB {
		availableKB = totalKB
	}

	return totalKB * 1024, availableKB * 1024, nil
}

func readUptimeSeconds() (int64, error) {
	content, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(content))
	if len(fields) < 1 {
		return 0, fmt.Errorf("invalid /proc/uptime content")
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse /proc/uptime: %w", err)
	}
	if value < 0 {
		return 0, nil
	}
	return int64(value), nil
}

