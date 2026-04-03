package handlers

import (
	"testing"

	"h2v2/internal/repository"
	runtimecore "h2v2/internal/runtime"
)

func TestMergeCredentialsWithCurrentPreservesSensitiveFields(t *testing.T) {
	current := []repository.Credential{
		{Protocol: repository.ProtocolHY2, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", Secret: "old-secret"},
		{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
	}
	next := []repository.Credential{
		{Protocol: repository.ProtocolHY2, Identity: "", Secret: ""},
		{Protocol: repository.ProtocolVLESS, Identity: ""},
	}

	merged := mergeCredentialsWithCurrent(next, current)
	if len(merged) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(merged))
	}
	if merged[0].Secret != "old-secret" {
		t.Fatalf("expected hy2 secret to be preserved")
	}
	if merged[0].Identity != "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" {
		t.Fatalf("expected hy2 identity to be preserved")
	}
	if merged[1].Identity != "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" {
		t.Fatalf("expected vless uuid to be preserved")
	}
}

func TestResolveUnifiedAccessQRValueUsesExactProtocolURI(t *testing.T) {
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolHY2, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", Secret: "secret"},
			{Protocol: repository.ProtocolVLESS, Identity: "ae299911-bf1c-45d4-a6f5-23395f8f731a"},
		},
	}
	artifacts := map[repository.Protocol]runtimecore.UserArtifacts{
		repository.ProtocolHY2: {
			Protocol:  repository.ProtocolHY2,
			AccessURI: "hy2://secret@example.com:443?sni=cdn.example.com#demo",
		},
		repository.ProtocolVLESS: {
			Protocol:  repository.ProtocolVLESS,
			AccessURI: "vless://ae299911-bf1c-45d4-a6f5-23395f8f731a@example.com:443?type=tcp#demo",
		},
	}

	hy2Value := resolveUnifiedAccessQRValue(user, artifacts, "hy2")
	if hy2Value != artifacts[repository.ProtocolHY2].AccessURI {
		t.Fatalf("expected hy2 qr value to match access uri exactly, got %q", hy2Value)
	}

	vlessValue := resolveUnifiedAccessQRValue(user, artifacts, "vless")
	if vlessValue != artifacts[repository.ProtocolVLESS].AccessURI {
		t.Fatalf("expected vless qr value to match access uri exactly, got %q", vlessValue)
	}
}
