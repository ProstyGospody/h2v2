package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Hy2Traffic struct {
	TxBytes int64
	RxBytes int64
}

type Hy2OnlineSummary struct {
	TotalConnections     int64
	TCPConnections       int64
	UDPConnections       int64
	BreakdownAvailable   bool
}

type HysteriaClient struct {
	BaseURL string
	Secret  string
	Client  *http.Client
}

func NewHysteriaClient(baseURL string, secret string) *HysteriaClient {
	return &HysteriaClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Secret:  secret,
		Client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *HysteriaClient) FetchTraffic(ctx context.Context) (map[string]Hy2Traffic, error) {
	payload, err := c.getJSON(ctx, "/traffic")
	if err != nil {
		return nil, err
	}

	out := map[string]Hy2Traffic{}
	for identity, val := range findStatsObject(payload, "users") {
		item, ok := val.(map[string]any)
		if !ok {
			continue
		}
		out[identity] = Hy2Traffic{
			TxBytes: pickInt64(item, "tx", "tx_bytes", "up", "upload"),
			RxBytes: pickInt64(item, "rx", "rx_bytes", "down", "download"),
		}
	}
	return out, nil
}

func (c *HysteriaClient) FetchOnline(ctx context.Context) (map[string]int, error) {
	online, _, err := c.FetchOnlineStats(ctx)
	if err != nil {
		return nil, err
	}
	return online, nil
}

func (c *HysteriaClient) FetchOnlineStats(ctx context.Context) (map[string]int, Hy2OnlineSummary, error) {
	payload, err := c.getJSON(ctx, "/online")
	if err != nil {
		return nil, Hy2OnlineSummary{}, err
	}

	online, summary := parseOnlinePayload(payload)
	return online, summary, nil
}

func (c *HysteriaClient) Kick(ctx context.Context, identity string) error {
	body := []string{identity}
	encoded, _ := json.Marshal(body)
	url := c.BaseURL + "/kick"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Secret != "" {
		req.Header.Set("Authorization", c.Secret)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("hy2 kick failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *HysteriaClient) getJSON(ctx context.Context, path string) (map[string]any, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", c.Secret)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("hy2 stats call failed path=%s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func findStatsObject(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	if nested, ok := payload[key].(map[string]any); ok {
		return nested
	}
	if nested, ok := payload["data"].(map[string]any); ok {
		if target, ok := nested[key].(map[string]any); ok {
			return target
		}
	}
	if users, ok := payload["users"].(map[string]any); ok {
		return users
	}
	return payload
}

func pickInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if parsed, ok := toInt64(val); ok {
				return parsed
			}
		}
	}
	return 0
}

func pickInt64Found(m map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		val, ok := m[key]
		if !ok {
			continue
		}
		parsed, ok := toInt64(val)
		if !ok {
			continue
		}
		return parsed, true
	}
	return 0, false
}

func parseOnlinePayload(payload map[string]any) (map[string]int, Hy2OnlineSummary) {
	users := map[string]int{}
	summary := Hy2OnlineSummary{}

	for identity, value := range findStatsObject(payload, "users") {
		total, tcp, udp, hasBreakdown := parseOnlineUser(value)
		if total < 0 {
			total = 0
		}
		users[identity] = int(total)
		summary.TotalConnections += total
		if hasBreakdown {
			summary.BreakdownAvailable = true
			if tcp > 0 {
				summary.TCPConnections += tcp
			}
			if udp > 0 {
				summary.UDPConnections += udp
			}
		}
	}

	return users, summary
}

func parseOnlineUser(value any) (int64, int64, int64, bool) {
	switch typed := value.(type) {
	case map[string]any:
		total, hasTotal := pickInt64Found(typed, "online", "count", "connections", "total", "total_connections")
		tcp, hasTCP := pickInt64Found(
			typed,
			"connections_tcp",
			"tcp_connections",
			"tcp_connection_count",
			"tcp_count",
			"tcp",
		)
		udp, hasUDP := pickInt64Found(
			typed,
			"connections_udp",
			"udp_connections",
			"udp_connection_count",
			"udp_count",
			"udp",
		)

		listTotal := int64(0)
		listTCP := int64(0)
		listUDP := int64(0)
		listHasBreakdown := false
		for _, key := range []string{"connections", "sessions"} {
			raw, ok := typed[key]
			if !ok {
				continue
			}
			items, ok := raw.([]any)
			if !ok {
				continue
			}
			currentTotal, currentTCP, currentUDP, currentBreakdown := parseOnlineConnectionList(items)
			listTotal += currentTotal
			listTCP += currentTCP
			listUDP += currentUDP
			listHasBreakdown = listHasBreakdown || currentBreakdown
		}

		total = maxInt64(total, 0)
		tcp = maxInt64(tcp, 0)
		udp = maxInt64(udp, 0)
		listTotal = maxInt64(listTotal, 0)
		listTCP = maxInt64(listTCP, 0)
		listUDP = maxInt64(listUDP, 0)

		breakdownAvailable := false
		if hasTCP || hasUDP {
			breakdownAvailable = true
		} else if listHasBreakdown {
			breakdownAvailable = true
			tcp = listTCP
			udp = listUDP
		}

		if !hasTotal {
			if hasTCP || hasUDP {
				if tcp+udp > 0 {
					total = tcp + udp
				} else if listTotal > 0 {
					total = listTotal
				}
			} else if listTotal > 0 {
				total = listTotal
			} else if tcp+udp > 0 {
				total = tcp + udp
			}
		}
		if breakdownAvailable && total < tcp+udp {
			total = tcp + udp
		}

		return total, tcp, udp, breakdownAvailable
	case []any:
		return parseOnlineConnectionList(typed)
	default:
		if n, ok := toInt64(typed); ok {
			if n < 0 {
				n = 0
			}
			return n, 0, 0, false
		}
	}

	return 0, 0, 0, false
}

func parseOnlineConnectionList(items []any) (int64, int64, int64, bool) {
	total := int64(len(items))
	tcp := int64(0)
	udp := int64(0)
	hasBreakdown := false

	for _, item := range items {
		protocol, ok := parseConnectionProtocol(item)
		if !ok {
			continue
		}
		hasBreakdown = true
		switch protocol {
		case "tcp":
			tcp++
		case "udp":
			udp++
		}
	}

	return total, tcp, udp, hasBreakdown
}

func parseConnectionProtocol(item any) (string, bool) {
	switch typed := item.(type) {
	case string:
		return normalizeProtocol(typed)
	case map[string]any:
		for _, key := range []string{"protocol", "network", "proto", "transport", "type"} {
			raw, ok := typed[key]
			if !ok {
				continue
			}
			value, ok := raw.(string)
			if !ok {
				continue
			}
			if normalized, ok := normalizeProtocol(value); ok {
				return normalized, true
			}
		}
	}
	return "", false
}

func normalizeProtocol(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "tcp"):
		return "tcp", true
	case strings.Contains(value, "udp"), strings.Contains(value, "quic"):
		return "udp", true
	default:
		return "", false
	}
}

func toInt64(v any) (int64, bool) {
	switch typed := v.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err == nil {
			return n, true
		}
		return 0, false
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}
