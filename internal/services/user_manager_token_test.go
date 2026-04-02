package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

func TestSubscriptionTokenV2RoundTrip(t *testing.T) {
	token := buildSubscriptionTokenV2("secret-value", "user-subject", 3)
	subject, version, ok := parseSubscriptionTokenV2("secret-value", token)
	if !ok {
		t.Fatalf("expected token to be valid")
	}
	if subject != "user-subject" || version != 3 {
		t.Fatalf("unexpected token payload: subject=%s version=%d", subject, version)
	}
}

func TestSubscriptionTokenV2RejectsTampering(t *testing.T) {
	token := buildSubscriptionTokenV2("secret-value", "user-subject", 1)
	if _, _, ok := parseSubscriptionTokenV2("wrong-secret", token); ok {
		t.Fatalf("expected token with wrong secret to be rejected")
	}
	tampered := token + "x"
	if _, _, ok := parseSubscriptionTokenV2("secret-value", tampered); ok {
		t.Fatalf("expected tampered token to be rejected")
	}
}

func TestBuildSubscriptionRenderFormats(t *testing.T) {
	expireAt := time.Now().UTC().Add(24 * time.Hour)
	user := repository.UserWithCredentials{
		User: repository.User{
			ID:                 "user-1",
			Name:               "demo",
			Enabled:            true,
			TrafficLimitBytes:  4096,
			TrafficUsedTxBytes: 100,
			TrafficUsedRxBytes: 200,
			ExpireAt:           &expireAt,
		},
	}

	artifacts := []runtimecore.UserArtifacts{
		{
			Protocol:   repository.ProtocolHY2,
			AccessURI:  "hy2://secret@example.com:443",
			ClashNode:  "- name: hy2\n  type: hysteria2",
			SingBoxNode: map[string]any{"type": "hysteria2"},
		},
		{
			Protocol:   repository.ProtocolVLESS,
			AccessURI:  "vless://uuid@example.com:443?type=tcp#demo",
			ClashNode:  "- name: vless\n  type: vless",
			SingBoxNode: map[string]any{"type": "vless"},
		},
	}

	uri := buildSubscriptionRender(user, artifacts, "uri")
	if !strings.Contains(string(uri.Body), "hy2://") || !strings.Contains(string(uri.Body), "vless://") {
		t.Fatalf("uri render is missing expected links: %s", string(uri.Body))
	}
	if uri.Headers["Profile-Title"] != "demo" {
		t.Fatalf("expected Profile-Title header to be set")
	}

	clash := buildSubscriptionRender(user, artifacts, "clash")
	if clash.ContentType != "application/yaml; charset=utf-8" {
		t.Fatalf("unexpected clash content type: %s", clash.ContentType)
	}
	if !strings.Contains(string(clash.Body), "proxies:") {
		t.Fatalf("expected clash payload to contain proxies section")
	}

	singbox := buildSubscriptionRender(user, artifacts, "singbox")
	if singbox.ContentType != "application/json; charset=utf-8" {
		t.Fatalf("unexpected singbox content type: %s", singbox.ContentType)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(singbox.Body, &payload); err != nil {
		t.Fatalf("singbox payload is not valid json: %v", err)
	}
	if _, ok := payload["outbounds"]; !ok {
		t.Fatalf("expected singbox payload to contain outbounds")
	}
}
