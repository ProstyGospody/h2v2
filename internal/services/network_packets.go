package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ProtocolPacketSnapshot struct {
	TCPPackets int64
	UDPPackets int64
	CollectedAt time.Time
}

func ReadProtocolPacketSnapshot() (ProtocolPacketSnapshot, error) {
	content, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return ProtocolPacketSnapshot{}, fmt.Errorf("read /proc/net/snmp: %w", err)
	}

	tcpIn, tcpOut, okTCP := int64(0), int64(0), false
	udpIn, udpOut, okUDP := int64(0), int64(0), false

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for idx := 0; idx+1 < len(lines); idx++ {
		headerLine := strings.TrimSpace(lines[idx])
		valueLine := strings.TrimSpace(lines[idx+1])
		headerFields := strings.Fields(headerLine)
		valueFields := strings.Fields(valueLine)
		if len(headerFields) < 2 || len(valueFields) < 2 {
			continue
		}

		protocol := strings.TrimSuffix(headerFields[0], ":")
		valueProtocol := strings.TrimSuffix(valueFields[0], ":")
		if protocol == "" || protocol != valueProtocol {
			continue
		}

		stats := map[string]int64{}
		limit := len(headerFields)
		if len(valueFields) < limit {
			limit = len(valueFields)
		}
		for statIdx := 1; statIdx < limit; statIdx++ {
			value, parseErr := strconv.ParseInt(valueFields[statIdx], 10, 64)
			if parseErr != nil {
				continue
			}
			stats[headerFields[statIdx]] = value
		}

		switch protocol {
		case "Tcp":
			inSegs, hasIn := stats["InSegs"]
			outSegs, hasOut := stats["OutSegs"]
			if hasIn && hasOut {
				tcpIn = inSegs
				tcpOut = outSegs
				okTCP = true
			}
		case "Udp":
			inDatagrams, hasIn := stats["InDatagrams"]
			outDatagrams, hasOut := stats["OutDatagrams"]
			if hasIn && hasOut {
				udpIn = inDatagrams
				udpOut = outDatagrams
				okUDP = true
			}
		}
	}

	if !okTCP && !okUDP {
		return ProtocolPacketSnapshot{}, fmt.Errorf("tcp/udp counters not found in /proc/net/snmp")
	}

	return ProtocolPacketSnapshot{
		TCPPackets:  maxInt64(0, tcpIn+tcpOut),
		UDPPackets:  maxInt64(0, udpIn+udpOut),
		CollectedAt: time.Now().UTC(),
	}, nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
