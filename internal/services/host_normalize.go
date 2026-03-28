package services

import (
	"net"
	"net/url"
	"strings"
)

func NormalizeHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}

	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			host = parsed.Host
		}
	}

	if idx := strings.IndexAny(host, "/?#"); idx >= 0 {
		host = host[:idx]
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	host = strings.TrimSpace(strings.Trim(host, "[]"))
	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(host)
}