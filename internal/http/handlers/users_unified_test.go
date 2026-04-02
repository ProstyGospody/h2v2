package handlers

import (
	"testing"

	"h2v2/internal/repository"
)

func TestMergeCredentialsWithCurrentPreservesSensitiveFields(t *testing.T) {
	current := []repository.Credential{
		{Protocol: repository.ProtocolHY2, Identity: "demo", Secret: "old-secret"},
		{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
	}
	next := []repository.Credential{
		{Protocol: repository.ProtocolHY2, Identity: "demo", Secret: ""},
		{Protocol: repository.ProtocolVLESS, Identity: ""},
	}

	merged := mergeCredentialsWithCurrent(next, current)
	if len(merged) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(merged))
	}
	if merged[0].Secret != "old-secret" {
		t.Fatalf("expected hy2 secret to be preserved")
	}
	if merged[1].Identity != "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" {
		t.Fatalf("expected vless uuid to be preserved")
	}
}
