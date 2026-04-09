package core

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/services"
)

const (
	singBoxV2RayAPIBasePort = 39000
	singBoxV2RayAPIPortSpan = 10000
)

func (s *Service) SyncRuntimeUsage(ctx context.Context) error {
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return err
	}

	var failures []string
	for _, server := range servers {
		collectedAt := time.Now().UTC()
		usageByUsername, err := services.QuerySingBoxUserTraffic(ctx, runtimeStatsListen(server))
		if err != nil {
			s.logger.Warn("runtime usage query failed", "server_id", server.ID, "error", err)
			failures = append(failures, server.ID+": "+err.Error())
			continue
		}
		if err := s.store.SyncServerRuntimeUsage(
			ctx,
			server.ID,
			s.runtimeServiceInstanceID(ctx, server),
			usageByUsername,
			collectedAt,
		); err != nil {
			s.logger.Warn("runtime usage sync failed", "server_id", server.ID, "error", err)
			failures = append(failures, server.ID+": "+err.Error())
		}
	}

	if len(failures) > 0 && len(failures) == len(servers) {
		return fmt.Errorf("runtime usage sync failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *Service) buildExperimentalSection(server Server, usernames []string) map[string]any {
	usernames = uniqueRuntimeStatUsers(usernames)
	if len(usernames) == 0 {
		return nil
	}
	return map[string]any{
		"v2ray_api": map[string]any{
			"listen": runtimeStatsListen(server),
			"stats": map[string]any{
				"enabled": true,
				"users":   usernames,
			},
		},
	}
}

func (s *Service) runtimeServiceInstanceID(ctx context.Context, server Server) string {
	if s == nil || s.serviceManager == nil {
		return ""
	}
	serviceName := strings.TrimSpace(server.SingBoxServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(s.cfg.SingBoxServiceName)
	}
	if serviceName == "" {
		return ""
	}
	details, err := s.serviceManager.Status(ctx, serviceName)
	if err != nil {
		return ""
	}
	if details.MainPID <= 0 {
		return ""
	}
	return serviceName + ":" + strconv.FormatInt(details.MainPID, 10)
}

func runtimeStatsListen(server Server) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(server.ID)))
	port := singBoxV2RayAPIBasePort + int(hasher.Sum32()%singBoxV2RayAPIPortSpan)
	return "127.0.0.1:" + strconv.Itoa(port)
}

func uniqueRuntimeStatUsers(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
