package runtime

import (
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"

	"h2v2/internal/repository"
)

func firstNonEmpty(values ...string) string {
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value != "" {
			return value
		}
	}
	return ""
}

func userCredential(user repository.UserWithCredentials, protocol repository.Protocol) (repository.Credential, bool) {
	for _, item := range user.Credentials {
		if item.Protocol != protocol {
			continue
		}
		normalized := item
		normalized.Identity = strings.TrimSpace(item.Identity)
		normalized.Secret = strings.TrimSpace(item.Secret)
		normalized.DataJSON = strings.TrimSpace(item.DataJSON)
		if normalized.Identity == "" {
			continue
		}
		return normalized, true
	}
	return repository.Credential{}, false
}

func renderVLESSClientConfig(accessURI string, clashNode string, singBoxNode map[string]any) string {
	lines := make([]string, 0, 12)
	uri := strings.TrimSpace(accessURI)
	if uri != "" {
		lines = append(lines, "uri:")
		lines = append(lines, uri)
	}

	clash := strings.TrimSpace(clashNode)
	if clash != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "clash:")
		lines = append(lines, clash)
	}

	if len(singBoxNode) > 0 {
		encoded, err := json.MarshalIndent(singBoxNode, "", "  ")
		if err == nil {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "singbox:")
			lines = append(lines, string(encoded))
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func parseJSONMap(raw string) map[string]any {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]any{}
	}
	decoded := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return map[string]any{}
	}
	if decoded == nil {
		return map[string]any{}
	}
	return decoded
}

func readString(source map[string]any, key string) string {
	if source == nil {
		return ""
	}
	raw, ok := source[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return firstNonEmpty(typed...)
	case []any:
		for _, item := range typed {
			if value := stringFromAny(item); value != "" {
				return value
			}
		}
		return ""
	default:
		return stringFromAny(typed)
	}
}

func readStringSlice(source map[string]any, key string) []string {
	if source == nil {
		return nil
	}
	raw, ok := source[key]
	if !ok || raw == nil {
		return nil
	}
	items := make([]string, 0, 4)
	appendValue := func(value string) {
		for _, item := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}
	switch typed := raw.(type) {
	case []string:
		for _, item := range typed {
			appendValue(item)
		}
	case []any:
		for _, item := range typed {
			appendValue(stringFromAny(item))
		}
	default:
		appendValue(stringFromAny(typed))
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func readInt(source map[string]any, key string, fallback int) int {
	if source == nil {
		return fallback
	}
	raw, ok := source[key]
	if !ok || raw == nil {
		return fallback
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
		if parsed, err := typed.Float64(); err == nil {
			return int(parsed)
		}
		return fallback
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return fallback
		}
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return int(parsed)
		}
		return fallback
	default:
		return fallback
	}
}

func normalizeHostOnly(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil && strings.TrimSpace(parsed.Host) != "" {
			value = strings.TrimSpace(parsed.Host)
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(strings.TrimSpace(host), "[]")
	}
	if strings.Count(value, ":") == 1 {
		host, portRaw, hasPort := strings.Cut(value, ":")
		if hasPort {
			if port, err := strconv.Atoi(strings.TrimSpace(portRaw)); err == nil && port > 0 {
				return strings.Trim(strings.TrimSpace(host), "[]")
			}
		}
	}
	if slash := strings.Index(value, "/"); slash >= 0 {
		value = value[:slash]
	}
	return strings.Trim(strings.TrimSpace(value), "[]")
}

func normalizePublicEndpointHost(raw string) string {
	host := normalizeHostOnly(raw)
	if host == "" {
		return ""
	}

	if parsedIP := net.ParseIP(host); parsedIP != nil {
		if parsedIP.IsLoopback() || parsedIP.IsUnspecified() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() {
			return ""
		}
		return parsedIP.String()
	}

	lowered := strings.ToLower(strings.TrimSpace(host))
	if lowered == "" {
		return ""
	}
	if lowered == "localhost" || strings.HasSuffix(lowered, ".localhost") || strings.HasSuffix(lowered, ".local") {
		return ""
	}
	return lowered
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return strings.TrimSpace(typed.String())
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}
