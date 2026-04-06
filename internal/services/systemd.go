package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ServiceCapabilities describes which control actions a service supports.
type ServiceCapabilities struct {
	// SupportsReload indicates the service has an ExecReload directive and
	// handles SIGHUP / systemctl reload gracefully. Most simple daemons
	// (including sing-box) do NOT set this; use Restart instead.
	SupportsReload bool
}

type ServiceManager struct {
	SystemctlPath        string
	SudoPath             string
	JournalctlPath       string
	ManagedServices      map[string]struct{}
	serviceCapabilities  map[string]ServiceCapabilities
	UseSudo              bool
	CommandTimeout       time.Duration
	MaxLogLinesLimit     int
}

type ServiceDetails struct {
	Name       string            `json:"name"`
	Active     string            `json:"active"`
	SubState   string            `json:"sub_state"`
	MainPID    int64             `json:"main_pid"`
	Uptime     string            `json:"uptime"`
	Raw        map[string]string `json:"raw"`
	LastLogs   []string          `json:"last_logs,omitempty"`
	Version    string            `json:"version,omitempty"`
	CheckedAt  time.Time         `json:"checked_at"`
	StatusText string            `json:"status_text"`
}

func NewServiceManager(systemctlPath string, sudoPath string, journalctlPath string, services []string, commandTimeout time.Duration) *ServiceManager {
	managed := make(map[string]struct{}, len(services))
	for _, svc := range services {
		svc = strings.TrimSpace(svc)
		if svc == "" {
			continue
		}
		managed[svc] = struct{}{}
	}
	return &ServiceManager{
		SystemctlPath:       systemctlPath,
		SudoPath:            sudoPath,
		JournalctlPath:      journalctlPath,
		ManagedServices:     managed,
		serviceCapabilities: make(map[string]ServiceCapabilities),
		UseSudo:             true,
		CommandTimeout:      commandTimeout,
		MaxLogLinesLimit:    200,
	}
}

// Capabilities returns the declared capabilities for a service.
// The default (no entry) is conservative: no reload support.
func (m *ServiceManager) Capabilities(service string) ServiceCapabilities {
	if caps, ok := m.serviceCapabilities[service]; ok {
		return caps
	}
	return ServiceCapabilities{SupportsReload: false}
}

// SetCapabilities configures the capabilities for a named service.
func (m *ServiceManager) SetCapabilities(service string, caps ServiceCapabilities) {
	if m.serviceCapabilities == nil {
		m.serviceCapabilities = make(map[string]ServiceCapabilities)
	}
	m.serviceCapabilities[service] = caps
}

func (m *ServiceManager) isAllowed(service string) bool {
	_, ok := m.ManagedServices[service]
	return ok
}

func (m *ServiceManager) command(ctx context.Context, bin string, args ...string) ([]byte, error) {
	if m.CommandTimeout <= 0 {
		m.CommandTimeout = 30 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, m.CommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, bin, args...)
	return cmd.CombinedOutput()
}

func (m *ServiceManager) runSystemctl(ctx context.Context, args ...string) ([]byte, error) {
	if m.UseSudo {
		fullArgs := append([]string{m.SystemctlPath}, args...)
		return m.command(ctx, m.SudoPath, fullArgs...)
	}
	return m.command(ctx, m.SystemctlPath, args...)
}

func (m *ServiceManager) Status(ctx context.Context, service string) (ServiceDetails, error) {
	if !m.isAllowed(service) {
		return ServiceDetails{}, fmt.Errorf("service %s is not allowed", service)
	}

	out, err := m.runSystemctl(
		ctx,
		"show",
		service,
		"--property=ActiveState",
		"--property=SubState",
		"--property=MainPID",
		"--property=ActiveEnterTimestamp",
	)
	if err != nil {
		return ServiceDetails{}, fmt.Errorf("systemctl show failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	rawMap := map[string]string{}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			continue
		}
		rawMap[pair[0]] = pair[1]
	}

	pid, _ := strconv.ParseInt(rawMap["MainPID"], 10, 64)
	statusText := rawMap["ActiveState"]
	if sub := rawMap["SubState"]; sub != "" {
		statusText = statusText + "/" + sub
	}
	return ServiceDetails{
		Name:       service,
		Active:     rawMap["ActiveState"],
		SubState:   rawMap["SubState"],
		MainPID:    pid,
		Uptime:     rawMap["ActiveEnterTimestamp"],
		Raw:        rawMap,
		CheckedAt:  time.Now().UTC(),
		StatusText: statusText,
	}, nil
}

func (m *ServiceManager) Restart(ctx context.Context, service string) error {
	if !m.isAllowed(service) {
		return fmt.Errorf("service %s is not allowed", service)
	}
	out, err := m.runSystemctl(ctx, "restart", service)
	if err != nil {
		return fmt.Errorf("restart failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *ServiceManager) Reload(ctx context.Context, service string) error {
	if !m.isAllowed(service) {
		return fmt.Errorf("service %s is not allowed", service)
	}
	out, err := m.runSystemctl(ctx, "reload", service)
	if err != nil {
		return fmt.Errorf("reload failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *ServiceManager) Logs(ctx context.Context, service string, lines int) ([]string, error) {
	if !m.isAllowed(service) {
		return nil, fmt.Errorf("service %s is not allowed", service)
	}
	if lines <= 0 {
		lines = 50
	}
	if m.MaxLogLinesLimit > 0 && lines > m.MaxLogLinesLimit {
		lines = m.MaxLogLinesLimit
	}
	args := []string{"-u", service, "-n", strconv.Itoa(lines), "--no-pager", "--output=short-iso"}
	var out []byte
	var err error
	if m.UseSudo {
		full := append([]string{m.JournalctlPath}, args...)
		out, err = m.command(ctx, m.SudoPath, full...)
	} else {
		out, err = m.command(ctx, m.JournalctlPath, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("journalctl failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	logLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(logLines) == 1 && logLines[0] == "" {
		return []string{}, nil
	}
	return logLines, nil
}

func (m *ServiceManager) ToJSON(details ServiceDetails) string {
	encoded, err := json.Marshal(details)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
