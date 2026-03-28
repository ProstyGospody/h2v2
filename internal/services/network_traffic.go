package services

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type NetworkTrafficSnapshot struct {
	RxBytes     int64
	TxBytes     int64
	CollectedAt time.Time
}

func ReadNetworkTrafficSnapshot() (NetworkTrafficSnapshot, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return NetworkTrafficSnapshot{}, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineIndex := 0
	var rxTotal int64
	var txTotal int64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineIndex++

		// Skip headers.
		if lineIndex <= 2 || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		if shouldIgnoreNetworkInterface(iface) {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) < 16 {
			continue
		}

		rx, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		tx, err := strconv.ParseInt(fields[8], 10, 64)
		if err != nil {
			continue
		}

		if rx > 0 {
			rxTotal += rx
		}
		if tx > 0 {
			txTotal += tx
		}
	}

	if err := scanner.Err(); err != nil {
		return NetworkTrafficSnapshot{}, fmt.Errorf("scan /proc/net/dev: %w", err)
	}

	return NetworkTrafficSnapshot{
		RxBytes:     rxTotal,
		TxBytes:     txTotal,
		CollectedAt: time.Now().UTC(),
	}, nil
}

func shouldIgnoreNetworkInterface(name string) bool {
	value := strings.ToLower(strings.TrimSpace(name))
	if value == "" || value == "lo" {
		return true
	}
	for _, prefix := range []string{"docker", "veth", "br-", "virbr", "zt"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
