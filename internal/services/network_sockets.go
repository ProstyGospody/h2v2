package services

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type ProtocolSocketSnapshot struct {
	TCPSockets  int64
	UDPSockets  int64
	CollectedAt time.Time
}

func ReadProtocolSocketSnapshot() (ProtocolSocketSnapshot, error) {
	tcpCount, tcpErr := readSocketCount("/proc/net/tcp")
	udpCount, udpErr := readSocketCount("/proc/net/udp")

	if tcpErr != nil && udpErr != nil {
		return ProtocolSocketSnapshot{}, fmt.Errorf("read proc socket counters: tcp=%v udp=%v", tcpErr, udpErr)
	}
	if tcpErr != nil {
		tcpCount = 0
	}
	if udpErr != nil {
		udpCount = 0
	}

	return ProtocolSocketSnapshot{
		TCPSockets:  maxInt64(0, tcpCount),
		UDPSockets:  maxInt64(0, udpCount),
		CollectedAt: time.Now().UTC(),
	}, nil
}

func readSocketCount(path string) (int64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) <= 1 {
		return 0, nil
	}

	var count int64
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		count++
	}
	return count, nil
}
