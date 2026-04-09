package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func DetectBinaryOutput(ctx context.Context, binaryPath string, args ...string) (string, error) {
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
	return strings.TrimSpace(string(out)), nil
}

func DetectBinaryVersion(ctx context.Context, binaryPath string, args ...string) (string, error) {
	line, err := DetectBinaryOutput(ctx, binaryPath, args...)
	if err != nil {
		return "", err
	}
	if idx := strings.IndexByte(line, '\n'); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line, nil
}

func DetectBinaryFeature(ctx context.Context, binaryPath string, feature string, args ...string) (bool, error) {
	output, err := DetectBinaryOutput(ctx, binaryPath, args...)
	if err != nil {
		return false, err
	}
	needle := strings.ToLower(strings.TrimSpace(feature))
	if needle == "" {
		return false, nil
	}
	return strings.Contains(strings.ToLower(output), needle), nil
}

