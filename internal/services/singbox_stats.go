package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	statscommand "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserTrafficUsage struct {
	UploadBytes   int64
	DownloadBytes int64
}

func QuerySingBoxUserTraffic(ctx context.Context, listen string) (map[string]UserTrafficUsage, error) {
	target := strings.TrimSpace(listen)
	if target == "" {
		return nil, fmt.Errorf("listen address is required")
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	queryCtx, queryCancel := context.WithTimeout(ctx, 3*time.Second)
	defer queryCancel()

	client := statscommand.NewStatsServiceClient(conn)
	resp, err := client.QueryStats(queryCtx, &statscommand.QueryStatsRequest{
		Patterns: []string{
			`^user>>>.*>>>traffic>>>uplink$`,
			`^user>>>.*>>>traffic>>>downlink$`,
		},
		Regexp: true,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]UserTrafficUsage)
	for _, item := range resp.GetStat() {
		username, direction, ok := parseSingBoxUserStat(item.GetName())
		if !ok {
			continue
		}
		usage := result[username]
		value := item.GetValue()
		if value < 0 {
			value = 0
		}
		switch direction {
		case "uplink":
			usage.UploadBytes = value
		case "downlink":
			usage.DownloadBytes = value
		default:
			continue
		}
		result[username] = usage
	}

	return result, nil
}

func parseSingBoxUserStat(raw string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ">>>")
	if len(parts) < 4 || parts[0] != "user" {
		return "", "", false
	}
	direction := strings.TrimSpace(parts[len(parts)-1])
	if direction != "uplink" && direction != "downlink" {
		return "", "", false
	}
	if strings.TrimSpace(parts[len(parts)-2]) != "traffic" {
		return "", "", false
	}
	username := strings.TrimSpace(strings.Join(parts[1:len(parts)-2], ">>>"))
	if username == "" {
		return "", "", false
	}
	return username, direction, true
}
