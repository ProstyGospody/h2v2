package runtime

import (
	"strings"
	"testing"
	"time"

	"h2v2/internal/repository"
)

func TestBuildXrayConfigFiltersDisabledAndExpiredUsers(t *testing.T) {
	inbounds := []repository.Inbound{
		{
			ID:        "vless-main",
			Name:      "VLESS Main",
			Protocol:  repository.ProtocolVLESS,
			Transport: "tcp",
			Security:  "reality",
			Host:      "example.com",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"flow":"xtls-rprx-vision","sni":"cdn.example.com","privateKey":"priv","sid":"ab12"}`,
		},
	}

	expiredAt := time.Now().UTC().Add(-time.Hour)
	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "ok", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
			},
		},
		{
			User: repository.User{ID: "u2", Name: "disabled", Enabled: false},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "ae299911-bf1c-45d4-a6f5-23395f8f731a"},
			},
		},
		{
			User: repository.User{ID: "u3", Name: "expired", Enabled: true, ExpireAt: &expiredAt},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2f71ef34-f01f-45d6-88f1-cd73c194f960"},
			},
		},
	}

	config, err := buildXrayConfig(inbounds, users)
	if err != nil {
		t.Fatalf("build xray config: %v", err)
	}
	inboundItems, ok := config["inbounds"].([]map[string]any)
	if !ok || len(inboundItems) == 0 {
		t.Fatalf("inbounds are missing in generated config")
	}
	settings, ok := inboundItems[0]["settings"].(map[string]any)
	if !ok {
		t.Fatalf("inbound settings are missing")
	}
	clients, ok := settings["clients"].([]map[string]any)
	if !ok {
		t.Fatalf("inbound clients list is missing")
	}
	if len(clients) != 1 {
		t.Fatalf("expected one active client, got %d", len(clients))
	}
	if clients[0]["id"] != "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" {
		t.Fatalf("unexpected client in generated config: %+v", clients[0])
	}
}

func TestBuildVLESSURIIncludesTransportAndRealityFields(t *testing.T) {
	uri := buildVLESSURI(
		repository.UserWithCredentials{User: repository.User{Name: "demo"}},
		repository.Credential{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		repository.Inbound{
			Protocol:  repository.ProtocolVLESS,
			Transport: "ws",
			Security:  "reality",
			Host:      "example.com",
			Port:      443,
		},
		map[string]any{
			"sni":  "cdn.example.com",
			"pbk":  "public-key",
			"sid":  "ab12",
			"path": "/ws",
			"fp":   "chrome",
		},
	)

	if !strings.HasPrefix(uri, "vless://2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad@example.com:443?") {
		t.Fatalf("unexpected uri prefix: %s", uri)
	}
	for _, needle := range []string{
		"type=ws",
		"security=reality",
		"sni=cdn.example.com",
		"pbk=public-key",
		"sid=ab12",
		"path=%2Fws",
		"fp=chrome",
	} {
		if !strings.Contains(uri, needle) {
			t.Fatalf("expected uri to contain %q, got %s", needle, uri)
		}
	}
}
