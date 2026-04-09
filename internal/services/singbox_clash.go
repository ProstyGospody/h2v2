package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type ClashTrafficSnapshot struct {
	UploadTotal    int64
	DownloadTotal  int64
	ConnectionCount int
}

func QuerySingBoxClashSnapshot(ctx context.Context, controller string) (ClashTrafficSnapshot, error) {
	target := strings.TrimSpace(controller)
	if target == "" {
		return ClashTrafficSnapshot{}, fmt.Errorf("controller address is required")
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return ClashTrafficSnapshot{}, fmt.Errorf("invalid controller address: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "http://"+target+"/connections", nil)
	if err != nil {
		return ClashTrafficSnapshot{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ClashTrafficSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ClashTrafficSnapshot{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload struct {
		DownloadTotal int64             `json:"downloadTotal"`
		UploadTotal   int64             `json:"uploadTotal"`
		Connections   []json.RawMessage `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ClashTrafficSnapshot{}, err
	}

	if payload.DownloadTotal < 0 {
		payload.DownloadTotal = 0
	}
	if payload.UploadTotal < 0 {
		payload.UploadTotal = 0
	}

	return ClashTrafficSnapshot{
		UploadTotal:    payload.UploadTotal,
		DownloadTotal:  payload.DownloadTotal,
		ConnectionCount: len(payload.Connections),
	}, nil
}
