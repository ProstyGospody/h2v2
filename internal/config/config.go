package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                 string
	ListenAddr          string
	PublicPanelURL      string
	SubscriptionPublicURL string
	PanelPublicHost     string
	PanelPublicPort     int
	StorageRoot         string
	SQLitePath          string
	RuntimeDir          string
	SessionCookieName   string
	CSRFCookieName      string
	CSRFHeaderName      string
	SessionTTL          time.Duration
	SecureCookies       bool
	InternalAuthToken   string
	RuntimePollInterval time.Duration
	ServicePollInterval time.Duration
	ManagedServices     []string
	SystemctlPath       string
	SudoPath            string
	JournalctlPath      string
	ServiceCommandTimeout time.Duration
	LogLinesMax         int
	RateLimitWindow     time.Duration
	RateLimitBurst      int
	SingBoxBinaryPath   string
	SingBoxConfigPath   string
	SingBoxServiceName  string
	HY2Domain           string
	HY2Port             int
	HY2CertPath         string
	HY2KeyPath          string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                 getEnv("APP_ENV", "production"),
		ListenAddr:          getEnv("PANEL_API_LISTEN_ADDR", "127.0.0.1:18080"),
		PublicPanelURL:      strings.TrimRight(getEnv("PANEL_PUBLIC_URL", ""), "/"),
		SubscriptionPublicURL: strings.TrimRight(getEnv("SUBSCRIPTION_PUBLIC_URL", ""), "/"),
		PanelPublicHost:     getEnv("PANEL_PUBLIC_HOST", "127.0.0.1"),
		PanelPublicPort:     getEnvInt("PANEL_PUBLIC_PORT", 8443),
		StorageRoot:         getEnv("PANEL_STORAGE_ROOT", "/var/lib/h2v2"),
		SQLitePath:          getEnv("PANEL_SQLITE_PATH", ""),
		RuntimeDir:          getEnv("PANEL_RUNTIME_DIR", "/run/h2v2"),
		SessionCookieName:   getEnv("SESSION_COOKIE_NAME", "pp_session"),
		CSRFCookieName:      getEnv("CSRF_COOKIE_NAME", "pp_csrf"),
		CSRFHeaderName:      getEnv("CSRF_HEADER_NAME", "X-CSRF-Token"),
		SessionTTL:          getEnvDuration("SESSION_TTL", 24*time.Hour),
		SecureCookies:       getEnvBool("SECURE_COOKIES", true),
		InternalAuthToken:   getEnv("INTERNAL_AUTH_TOKEN", ""),
		RuntimePollInterval: getEnvDuration("RUNTIME_POLL_INTERVAL", 20*time.Second),
		ServicePollInterval: getEnvDuration("SERVICE_POLL_INTERVAL", 60*time.Second),
		ManagedServices:     parseCSV(getEnv("MANAGED_SERVICES", "h2v2-api,h2v2-web,sing-box")),
		SystemctlPath:       getEnv("SYSTEMCTL_PATH", "/usr/bin/systemctl"),
		SudoPath:            getEnv("SUDO_PATH", "/usr/bin/sudo"),
		JournalctlPath:      getEnv("JOURNALCTL_PATH", "/usr/bin/journalctl"),
		ServiceCommandTimeout: getEnvDuration("SERVICE_COMMAND_TIMEOUT", 30*time.Second),
		LogLinesMax:         getEnvInt("SERVICE_LOG_LINES_MAX", 120),
		RateLimitWindow:     getEnvDuration("AUTH_RATE_LIMIT_WINDOW", 15*time.Minute),
		RateLimitBurst:      getEnvInt("AUTH_RATE_LIMIT_BURST", 10),
		SingBoxBinaryPath:   getEnv("SINGBOX_BINARY_PATH", "/usr/local/bin/sing-box"),
		SingBoxConfigPath:   getEnv("SINGBOX_CONFIG_PATH", "/etc/h2v2/sing-box/config.json"),
		SingBoxServiceName:  getEnv("SINGBOX_SERVICE_NAME", "sing-box"),
		HY2Domain:           getEnv("HY2_DOMAIN", ""),
		HY2Port:             getEnvInt("HY2_PORT", 443),
		HY2CertPath:         getEnv("HY2_CERT_PATH", "/etc/h2v2/hysteria/server.crt"),
		HY2KeyPath:          getEnv("HY2_KEY_PATH", "/etc/h2v2/hysteria/server.key"),
	}

	if strings.TrimSpace(cfg.StorageRoot) == "" {
		return Config{}, fmt.Errorf("PANEL_STORAGE_ROOT is required")
	}
	if strings.TrimSpace(cfg.SQLitePath) == "" {
		cfg.SQLitePath = filepath.Join(cfg.StorageRoot, "data", "h2v2.db")
	}
	if strings.TrimSpace(cfg.SQLitePath) == "" {
		return Config{}, fmt.Errorf("PANEL_SQLITE_PATH is required")
	}
	if strings.TrimSpace(cfg.RuntimeDir) == "" {
		return Config{}, fmt.Errorf("PANEL_RUNTIME_DIR is required")
	}
	if cfg.InternalAuthToken == "" {
		return Config{}, fmt.Errorf("INTERNAL_AUTH_TOKEN is required")
	}

	if cfg.PublicPanelURL == "" {
		cfg.PublicPanelURL = fmt.Sprintf("https://%s:%d", cfg.PanelPublicHost, cfg.PanelPublicPort)
	}
	if cfg.SubscriptionPublicURL == "" {
		cfg.SubscriptionPublicURL = cfg.PublicPanelURL
	}

	return cfg, nil
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		res = append(res, trimmed)
	}
	return res
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
