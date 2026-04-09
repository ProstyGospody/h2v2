package core

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"h2v2/internal/services"
)

const (
	singBoxV2RayAPIBasePort = 39000
	singBoxV2RayAPIPortSpan = 10000
	runtimeUsageCacheTTL    = 3 * time.Second
)

func (s *Service) ListUserRuntimeTraffic(ctx context.Context) (map[string]services.UserTrafficUsage, error) {
	s.runtimeUsageMu.Lock()
	if !s.runtimeUsageAt.IsZero() && time.Since(s.runtimeUsageAt) < runtimeUsageCacheTTL {
		cached := cloneRuntimeUsageMap(s.runtimeUsage)
		s.runtimeUsageMu.Unlock()
		return cached, nil
	}
	s.runtimeUsageMu.Unlock()

	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]services.UserTrafficUsage)
	var lastErr error
	for _, server := range servers {
		listen := configuredV2RayAPIListen(strings.TrimSpace(server.SingBoxConfigPath))
		if listen == "" {
			continue
		}
		usageByUser, err := services.QuerySingBoxUserTraffic(ctx, listen)
		if err != nil {
			lastErr = err
			continue
		}
		for username, usage := range usageByUser {
			current := result[username]
			current.UploadBytes += usage.UploadBytes
			current.DownloadBytes += usage.DownloadBytes
			result[username] = current
		}
	}

	if len(result) == 0 && lastErr != nil {
		return nil, lastErr
	}
	s.runtimeUsageMu.Lock()
	s.runtimeUsageAt = time.Now().UTC()
	s.runtimeUsage = cloneRuntimeUsageMap(result)
	s.runtimeUsageMu.Unlock()
	return result, nil
}

func (s *Service) overlayRuntimeTraffic(ctx context.Context, user User) User {
	usageByUser, err := s.ListUserRuntimeTraffic(ctx)
	if err != nil {
		return user
	}
	return applyRuntimeTrafficUsage(user, usageByUser)
}

func (s *Service) OverlayRuntimeTraffic(ctx context.Context, user User) User {
	return s.overlayRuntimeTraffic(ctx, user)
}

func applyRuntimeTrafficUsage(user User, usageByUser map[string]services.UserTrafficUsage) User {
	usage, ok := usageByUser[strings.TrimSpace(user.Username)]
	if !ok {
		return user
	}
	user.TrafficUsedUpBytes = usage.UploadBytes
	user.TrafficUsedDownBytes = usage.DownloadBytes
	return user
}

func ApplyRuntimeTrafficUsage(user User, usageByUser map[string]services.UserTrafficUsage) User {
	return applyRuntimeTrafficUsage(user, usageByUser)
}

func (s *Service) buildExperimentalSection(ctx context.Context, server Server, usernames []string) map[string]any {
	usernames = uniqueRuntimeStatUsers(usernames)
	if len(usernames) == 0 || !s.supportsV2RayAPI(ctx, server) {
		return nil
	}
	return map[string]any{
		"v2ray_api": map[string]any{
			"listen": plannedV2RayAPIListen(server),
			"stats": map[string]any{
				"enabled": true,
				"users":   usernames,
			},
		},
	}
}

func (s *Service) supportsV2RayAPI(ctx context.Context, server Server) bool {
	if configuredV2RayAPIListen(strings.TrimSpace(server.SingBoxConfigPath)) != "" {
		return true
	}
	binaryPath := strings.TrimSpace(s.defaultServer(server).SingBoxBinaryPath)
	if binaryPath == "" {
		return false
	}
	supported, err := services.DetectBinaryFeature(ctx, binaryPath, "with_v2ray_api", "version")
	if err == nil && supported {
		return true
	}
	if err != nil {
		return false
	}
	return false
}

func plannedV2RayAPIListen(server Server) string {
	current := configuredV2RayAPIListen(strings.TrimSpace(server.SingBoxConfigPath))
	if current != "" {
		return current
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(server.ID)))
	port := singBoxV2RayAPIBasePort + int(hasher.Sum32()%singBoxV2RayAPIPortSpan)
	return "127.0.0.1:" + strconv.Itoa(port)
}

func configuredV2RayAPIListen(configPath string) string {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return ""
	}
	payload, err := os.ReadFile(configPath)
	if err != nil || len(payload) == 0 {
		return ""
	}

	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return ""
	}

	experimental, _ := root["experimental"].(map[string]any)
	if experimental == nil {
		return ""
	}
	v2rayAPI, _ := experimental["v2ray_api"].(map[string]any)
	if v2rayAPI == nil {
		return ""
	}
	listen, _ := v2rayAPI["listen"].(string)
	return strings.TrimSpace(listen)
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

func cloneRuntimeUsageMap(input map[string]services.UserTrafficUsage) map[string]services.UserTrafficUsage {
	if len(input) == 0 {
		return map[string]services.UserTrafficUsage{}
	}
	result := make(map[string]services.UserTrafficUsage, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
