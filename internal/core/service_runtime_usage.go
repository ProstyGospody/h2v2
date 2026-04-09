package core

import (
	"context"
	"encoding/json"
	"fmt"
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
)

func (s *Service) SyncRuntimeUsage(ctx context.Context) error {
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return err
	}

	var failures []string
	supportedServers := 0
	for _, server := range servers {
		server = s.defaultServer(server)
		if !s.supportsV2RayAPI(ctx, server) {
			continue
		}
		supportedServers++
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

	if supportedServers > 0 && len(failures) > 0 && len(failures) == supportedServers {
		return fmt.Errorf("runtime usage sync failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *Service) buildExperimentalSection(ctx context.Context, server Server, usernames []string) map[string]any {
	server = s.defaultServer(server)
	usernames = uniqueRuntimeStatUsers(usernames)
	if len(usernames) == 0 {
		return nil
	}
	if !s.supportsV2RayAPI(ctx, server) {
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

func (s *Service) supportsV2RayAPI(ctx context.Context, server Server) bool {
	server = s.defaultServer(server)
	cacheKey := v2rayAPICacheKey(server)
	if cacheKey == "" {
		return false
	}

	now := time.Now().UTC()
	s.capabilityMu.Lock()
	cached, ok := s.v2rayAPIChecks[cacheKey]
	if ok && now.Sub(cached.CheckedAt) < s.capabilityTTL {
		s.capabilityMu.Unlock()
		return cached.Supported
	}
	s.capabilityMu.Unlock()

	supported := s.probeV2RayAPISupport(ctx, server)

	s.capabilityMu.Lock()
	s.v2rayAPIChecks[cacheKey] = capabilityCheck{
		CheckedAt: now,
		Supported: supported,
	}
	s.capabilityMu.Unlock()
	return supported
}

func (s *Service) invalidateServerCapabilities(server Server) {
	cacheKey := v2rayAPICacheKey(s.defaultServer(server))
	if cacheKey == "" {
		return
	}
	s.capabilityMu.Lock()
	delete(s.v2rayAPIChecks, cacheKey)
	s.capabilityMu.Unlock()
}

func (s *Service) probeV2RayAPISupport(ctx context.Context, server Server) bool {
	binary := strings.TrimSpace(server.SingBoxBinaryPath)
	if binary == "" {
		return false
	}
	if _, err := os.Stat(binary); err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("failed to inspect sing-box binary", "server_id", server.ID, "path", binary, "error", err)
		}
		return false
	}

	probeContent, err := json.Marshal(map[string]any{
		"log": map[string]any{
			"disabled": true,
		},
		"outbounds": []map[string]any{
			{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"experimental": map[string]any{
			"v2ray_api": map[string]any{
				"listen": runtimeStatsListen(server),
			},
		},
	})
	if err != nil {
		return false
	}

	if err := s.checkConfig(ctx, server, probeContent); err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "v2ray api is not included") || strings.Contains(message, "with_v2ray_api") {
			s.logger.Info("sing-box build does not support v2ray api", "server_id", server.ID, "path", binary)
			return false
		}
		s.logger.Warn("v2ray api capability probe failed", "server_id", server.ID, "error", err)
		return false
	}
	return true
}

func v2rayAPICacheKey(server Server) string {
	binary := strings.TrimSpace(server.SingBoxBinaryPath)
	if binary == "" {
		return ""
	}
	return binary
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
