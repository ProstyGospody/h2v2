package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func DetectBinaryVersion(ctx context.Context, binaryPath string, args ...string) (string, error) {
	if len(args) == 0 {
		args = []string{"version"}
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, binaryPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("exec %s %v: %w: %s", binaryPath, args, err, strings.TrimSpace(string(out)))
	}
	line := strings.TrimSpace(string(out))
	if idx := strings.IndexByte(line, '\n'); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line, nil
}

