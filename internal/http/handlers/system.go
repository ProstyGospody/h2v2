package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/http/render"
	"h2v2/internal/repository"
	"h2v2/internal/services"
)

const (
	systemTrendRetention         = 24 * time.Hour
	systemTrendMaxPoints         = 250000
	systemTrendDefaultLimit      = 50000
	systemTrendCollectorInterval = 3 * time.Second
	systemTrendDefaultStepSec    = 3
	systemTrendMaxStepSec        = 3600
)

type liveSystemMetrics struct {
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64     `json:"memory_used_bytes"`
	MemoryTotalBytes  int64     `json:"memory_total_bytes"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	NetworkRxBps      float64   `json:"network_rx_bps"`
	NetworkTxBps      float64   `json:"network_tx_bps"`
	TCPSockets        int64     `json:"tcp_sockets"`
	UDPSockets        int64     `json:"udp_sockets"`
	TCPPackets        int64     `json:"tcp_packets"`
	UDPPackets        int64     `json:"udp_packets"`
	TCPPacketsPerSec  float64   `json:"tcp_packets_per_sec"`
	UDPPacketsPerSec  float64   `json:"udp_packets_per_sec"`
	PacketsCollectedAt time.Time `json:"packets_collected_at"`
	PacketsSource     string    `json:"packets_source"`
	PacketsIsStale    bool      `json:"packets_is_stale"`
	CollectedAt       time.Time `json:"collected_at"`
	Source            string    `json:"source"`
	IsStale           bool      `json:"is_stale"`
}

type liveServiceStatus struct {
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	LastCheckAt time.Time `json:"last_check_at"`
	Source      string    `json:"source"`
	IsStale     bool      `json:"is_stale"`
	Error       string    `json:"error,omitempty"`
}

type liveHy2Overview struct {
	EnabledUsers                 int64     `json:"enabled_users"`
	TotalTxBytes                 int64     `json:"total_tx_bytes"`
	TotalRxBytes                 int64     `json:"total_rx_bytes"`
	OnlineCount                  int64     `json:"online_count"`
	ConnectionsTCP               int64     `json:"connections_tcp"`
	ConnectionsUDP               int64     `json:"connections_udp"`
	ConnectionsBreakdownAvailable bool      `json:"connections_breakdown_available"`
	CollectedAt                  time.Time `json:"collected_at"`
	Source                       string    `json:"source"`
	IsStale                      bool      `json:"is_stale"`
}

func (h *Handler) GetSystemLive(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	generatedAt := time.Now().UTC()
	errors := make([]string, 0, 3)

	snapshot, networkRx, networkTx, source, err := h.collectSystemMetrics(ctx)
	if err != nil {
		h.logger.Warn("failed to collect live system metrics", "error", err)
		errors = append(errors, "system metrics unavailable")
		snapshot = services.SystemMetrics{CollectedAt: generatedAt}
		source = "unavailable"
	}

	tcpPackets, udpPackets, packetCollectedAt, packetSource, packetErr := h.collectProtocolPacketMetrics()
	if packetErr != nil {
		h.logger.Warn("failed to collect protocol packet metrics", "error", packetErr)
		errors = append(errors, "protocol packet metrics unavailable")
	}
	tcpPacketsPerSec, udpPacketsPerSec := h.calculateProtocolPacketRates(tcpPackets, udpPackets, packetCollectedAt)
	tcpSockets, udpSockets, _, _, socketErr := h.collectProtocolSocketMetrics()
	if socketErr != nil {
		h.logger.Warn("failed to collect protocol socket metrics", "error", socketErr)
		errors = append(errors, "protocol socket metrics unavailable")
	}

	hy2, hy2Err := h.collectHy2Live(ctx)
	if hy2Err != "" {
		errors = append(errors, hy2Err)
	}

	serviceStatuses := h.collectProxyServiceStatuses(ctx)
	for _, service := range serviceStatuses {
		if service.Error != "" {
			errors = append(errors, service.ServiceName+" status unavailable")
		}
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"collected_at": generatedAt,
		"system": liveSystemMetrics{
			CPUUsagePercent:   snapshot.CPUUsagePercent,
			MemoryUsedBytes:   snapshot.MemoryUsedBytes,
			MemoryTotalBytes:  snapshot.MemoryTotalBytes,
			MemoryUsedPercent: snapshot.MemoryUsedPercent,
			UptimeSeconds:     snapshot.UptimeSeconds,
			NetworkRxBps:      networkRx,
			NetworkTxBps:      networkTx,
			TCPSockets:        tcpSockets,
			UDPSockets:        udpSockets,
			TCPPackets:        tcpPackets,
			UDPPackets:        udpPackets,
			TCPPacketsPerSec:  tcpPacketsPerSec,
			UDPPacketsPerSec:  udpPacketsPerSec,
			PacketsCollectedAt: packetCollectedAt,
			PacketsSource:     packetSource,
			PacketsIsStale:    packetSource == "unavailable" || time.Since(packetCollectedAt) > 15*time.Second,
			CollectedAt:       snapshot.CollectedAt,
			Source:            source,
			IsStale:           time.Since(snapshot.CollectedAt) > 15*time.Second,
		},
		"runtime":  hy2,
		"services": serviceStatuses,
		"errors":   errors,
	})
}

func (h *Handler) GetSystemHistory(w http.ResponseWriter, r *http.Request) {
	limit := parseSystemHistoryLimit(r.URL.Query().Get("limit"))
	window := parseSystemHistoryWindow(r.URL.Query().Get("window"))
	stepSeconds := parseSystemHistoryStepSeconds(r.URL.Query().Get("step"), window)
	to := time.Now().UTC()
	from := to.Add(-window)

	if h.repo == nil {
		render.JSON(w, http.StatusOK, map[string]any{"items": []systemTrendSample{}})
		return
	}

	snapshots, err := h.repo.ListSystemSnapshots(r.Context(), from, to, limit)
	if err != nil {
		h.logger.Warn("failed to load system history", "error", err)
		h.renderError(w, http.StatusInternalServerError, "runtime", "failed to load system history", nil)
		return
	}

	items := toSystemTrendSamples(snapshots, from, to, stepSeconds)
	render.JSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *Handler) collectSystemMetrics(ctx context.Context) (services.SystemMetrics, float64, float64, string, error) {
	if h.systemMetrics == nil {
		return services.SystemMetrics{}, 0, 0, "unavailable", fmt.Errorf("system metrics collector is not configured")
	}

	snapshot, err := h.systemMetrics.Snapshot(ctx)
	if err != nil {
		return services.SystemMetrics{}, 0, 0, "procfs", err
	}

	networkRx, networkTx, rateErr := h.collectProcNetworkRates()
	if rateErr != nil {
		h.logger.Debug("procfs network rate failed", "error", rateErr)
		return snapshot, 0, 0, "procfs", nil
	}
	return snapshot, networkRx, networkTx, "procfs", nil
}

func (h *Handler) collectProcNetworkRates() (float64, float64, error) {
	snapshot, err := services.ReadNetworkTrafficSnapshot()
	if err != nil {
		return 0, 0, err
	}

	collectedAt := snapshot.CollectedAt.UTC()
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	h.networkMu.Lock()
	defer h.networkMu.Unlock()

	prev := h.networkSample
	h.networkSample = networkByteSample{
		rxBytes:     snapshot.RxBytes,
		txBytes:     snapshot.TxBytes,
		collectedAt: collectedAt,
	}

	if prev.collectedAt.IsZero() || !collectedAt.After(prev.collectedAt) {
		return 0, 0, nil
	}

	seconds := collectedAt.Sub(prev.collectedAt).Seconds()
	if seconds <= 0 {
		return 0, 0, nil
	}

	rxDelta := snapshot.RxBytes - prev.rxBytes
	txDelta := snapshot.TxBytes - prev.txBytes
	if rxDelta < 0 {
		rxDelta = 0
	}
	if txDelta < 0 {
		txDelta = 0
	}

	return float64(rxDelta) / seconds, float64(txDelta) / seconds, nil
}

func (h *Handler) collectProtocolPacketMetrics() (int64, int64, time.Time, string, error) {
	snapshot, err := services.ReadProtocolPacketSnapshot()
	if err != nil {
		return 0, 0, time.Now().UTC(), "unavailable", err
	}
	return snapshot.TCPPackets, snapshot.UDPPackets, snapshot.CollectedAt, "procfs", nil
}

func (h *Handler) collectProtocolSocketMetrics() (int64, int64, time.Time, string, error) {
	snapshot, err := services.ReadProtocolSocketSnapshot()
	if err != nil {
		return 0, 0, time.Now().UTC(), "unavailable", err
	}
	return snapshot.TCPSockets, snapshot.UDPSockets, snapshot.CollectedAt, "procfs", nil
}

func (h *Handler) calculateProtocolPacketRates(tcpPackets int64, udpPackets int64, collectedAt time.Time) (float64, float64) {
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	h.protocolMu.Lock()
	defer h.protocolMu.Unlock()

	prev := h.protocolSample
	h.protocolSample = protocolPacketSample{
		tcpPackets:  tcpPackets,
		udpPackets:  udpPackets,
		collectedAt: collectedAt,
	}

	if prev.collectedAt.IsZero() || !collectedAt.After(prev.collectedAt) {
		return 0, 0
	}

	seconds := collectedAt.Sub(prev.collectedAt).Seconds()
	if seconds <= 0 {
		return 0, 0
	}

	tcpDelta := tcpPackets - prev.tcpPackets
	if tcpDelta < 0 {
		tcpDelta = 0
	}

	udpDelta := udpPackets - prev.udpPackets
	if udpDelta < 0 {
		udpDelta = 0
	}

	return float64(tcpDelta) / seconds, float64(udpDelta) / seconds
}

func (h *Handler) collectHy2Live(ctx context.Context) (liveHy2Overview, string) {
	if h.repo == nil {
		return liveHy2Overview{Source: "unavailable", IsStale: true, CollectedAt: time.Now().UTC()}, "runtime overview unavailable"
	}
	users, err := h.repo.ListUsers(ctx, 0, 0, nil)
	if err != nil {
		return liveHy2Overview{Source: "unavailable", IsStale: true, CollectedAt: time.Now().UTC()}, "runtime overview unavailable"
	}

	resp := liveHy2Overview{
		EnabledUsers:                 0,
		TotalTxBytes:                 0,
		TotalRxBytes:                 0,
		OnlineCount:                  0,
		ConnectionsTCP:               0,
		ConnectionsUDP:               0,
		ConnectionsBreakdownAvailable: false,
		CollectedAt:                  time.Now().UTC(),
		Source:                       "runtime",
		IsStale:                      false,
	}

	for _, user := range users {
		if user.Enabled {
			resp.EnabledUsers++
		}
		resp.TotalTxBytes += user.TrafficUsedTxBytes
		resp.TotalRxBytes += user.TrafficUsedRxBytes
		if user.OnlineCount > 0 {
			resp.OnlineCount += int64(user.OnlineCount)
		}
	}
	return resp, ""
}

func (h *Handler) collectProxyServiceStatuses(ctx context.Context) []liveServiceStatus {
	if h.serviceManager == nil || h.repo == nil {
		return []liveServiceStatus{}
	}
	targets := make([]string, 0, 1)
	for _, candidate := range []string{strings.TrimSpace(h.cfg.SingBoxServiceName)} {
		if candidate == "" {
			continue
		}
		if _, ok := h.serviceManager.ManagedServices[candidate]; ok {
			targets = append(targets, candidate)
		}
	}
	sort.Strings(targets)

	states := make([]liveServiceStatus, 0, len(targets))
	for _, name := range targets {
		details, err := h.serviceManager.Status(ctx, name)
		if err == nil {
			raw := h.serviceManager.ToJSON(details)
			_ = h.repo.UpsertServiceState(ctx, name, details.StatusText, nil, raw)
			states = append(states, liveServiceStatus{
				ServiceName: name,
				Status:      details.StatusText,
				LastCheckAt: details.CheckedAt,
				Source:      "live",
				IsStale:     false,
			})
			continue
		}

		cached, cacheErr := h.repo.GetServiceState(ctx, name)
		if cacheErr == nil {
			states = append(states, liveServiceStatus{
				ServiceName: name,
				Status:      cached.Status,
				LastCheckAt: cached.LastCheckAt,
				Source:      "cache",
				IsStale:     true,
				Error:       err.Error(),
			})
			continue
		}

		states = append(states, liveServiceStatus{
			ServiceName: name,
			Status:      "failed",
			LastCheckAt: time.Now().UTC(),
			Source:      "error",
			IsStale:     true,
			Error:       err.Error(),
		})
	}

	return states
}

func parseSystemHistoryLimit(raw string) int {
	limit := systemTrendDefaultLimit
	value := strings.TrimSpace(raw)
	if value == "" {
		return limit
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return limit
	}
	if parsed <= 0 {
		return limit
	}
	if parsed > systemTrendMaxPoints {
		return systemTrendMaxPoints
	}
	return parsed
}

func parseSystemHistoryWindow(raw string) time.Duration {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return systemTrendRetention
	}

	switch value {
	case "1h", "1hr", "hour":
		return time.Hour
	case "24h", "1d", "day":
		return 24 * time.Hour
	case "7d", "week", "1w":
		return 7 * 24 * time.Hour
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return systemTrendRetention
	}
	if parsed < time.Minute {
		return time.Minute
	}
	if parsed > 7*24*time.Hour {
		return 7 * 24 * time.Hour
	}
	return parsed
}

func parseSystemHistoryStepSeconds(raw string, window time.Duration) int {
	defaultStep := systemTrendDefaultStepSec
	if window >= 12*time.Hour {
		defaultStep = 30
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultStep
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultStep
	}
	if parsed > systemTrendMaxStepSec {
		return systemTrendMaxStepSec
	}
	return parsed
}

func toSystemTrendSamples(snapshots []repository.SystemSnapshot, from time.Time, to time.Time, stepSeconds int) []systemTrendSample {
	if len(snapshots) == 0 {
		return []systemTrendSample{}
	}
	if stepSeconds <= 1 {
		stepSeconds = 1
	}

	if from.IsZero() {
		from = snapshots[0].SnapshotAt.UTC()
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotAt.Before(snapshots[j].SnapshotAt)
	})

	if stepSeconds <= systemTrendDefaultStepSec {
		out := make([]systemTrendSample, 0, len(snapshots))
		for _, sample := range snapshots {
			out = append(out, systemTrendSample{
				Timestamp:         sample.SnapshotAt.UTC(),
				CPUUsagePercent:   sample.CPUUsagePercent,
				MemoryUsedPercent: sample.MemoryUsedPercent,
				NetworkRxBps:      sample.NetworkRxBps,
				NetworkTxBps:      sample.NetworkTxBps,
			})
		}
		return out
	}

	interval := time.Duration(stepSeconds) * time.Second
	buckets := make([]systemTrendSample, 0, len(snapshots))
	var current systemTrendSample
	var currentBucket time.Time
	hasCurrent := false

	for _, sample := range snapshots {
		ts := sample.SnapshotAt.UTC()
		bucket := ts.Truncate(interval)
		point := systemTrendSample{
			Timestamp:         ts,
			CPUUsagePercent:   sample.CPUUsagePercent,
			MemoryUsedPercent: sample.MemoryUsedPercent,
			NetworkRxBps:      sample.NetworkRxBps,
			NetworkTxBps:      sample.NetworkTxBps,
		}

		if !hasCurrent {
			current = point
			currentBucket = bucket
			hasCurrent = true
			continue
		}
		if bucket.Equal(currentBucket) {
			current = point
			continue
		}
		buckets = append(buckets, current)
		current = point
		currentBucket = bucket
	}

	if hasCurrent {
		buckets = append(buckets, current)
	}
	return buckets
}

func (h *Handler) StartSystemTrendCollector(ctx context.Context) {
	go func() {
		h.collectSystemTrendPoint(ctx)

		ticker := time.NewTicker(systemTrendCollectorInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.collectSystemTrendPoint(ctx)
			}
		}
	}()
}

func (h *Handler) collectSystemTrendPoint(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	snapshot, networkRx, networkTx, _, err := h.collectSystemMetrics(ctx)
	if err != nil {
		h.logger.Debug("system trend collection failed", "error", err)
		return
	}

	if h.repo == nil {
		return
	}

	_, err = h.repo.InsertSystemSnapshot(ctx, repository.SystemSnapshot{
		SnapshotAt:        snapshot.CollectedAt,
		CPUUsagePercent:   snapshot.CPUUsagePercent,
		MemoryUsedPercent: snapshot.MemoryUsedPercent,
		NetworkRxBps:      networkRx,
		NetworkTxBps:      networkTx,
	})
	if err != nil {
		h.logger.Debug("system trend persist failed", "error", err)
	}
}
