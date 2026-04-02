package runtime

import (
	"strings"
	"testing"
	"time"

	"h2v2/internal/repository"
)

func TestBuildSingBoxVLESSConfigFiltersInactiveUsers(t *testing.T) {
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
			ParamsJSON: `{"flow":"xtls-rprx-vision","sni":"cdn.example.com","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
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

	config, err := buildSingBoxVLESSConfig(inbounds, users, "panel.example.com")
	if err != nil {
		t.Fatalf("build sing-box config: %v", err)
	}
	inboundItems, ok := config["inbounds"].([]map[string]any)
	if !ok || len(inboundItems) != 1 {
		t.Fatalf("expected one inbound, got %+v", config["inbounds"])
	}
	item := inboundItems[0]
	if item["type"] != "vless" {
		t.Fatalf("unexpected inbound type: %+v", item["type"])
	}
	userEntries, ok := item["users"].([]map[string]any)
	if !ok || len(userEntries) != 1 {
		t.Fatalf("expected one active user entry, got %+v", item["users"])
	}
	if userEntries[0]["uuid"] != "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" {
		t.Fatalf("unexpected user entry: %+v", userEntries[0])
	}
	tls, ok := item["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls section is missing")
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok {
		t.Fatalf("reality section is missing")
	}
	if strings.TrimSpace(readString(reality, "private_key")) == "" {
		t.Fatalf("reality private key is missing in rendered inbound")
	}
}

func TestSingBoxAdapterBuildArtifactsUsesEnabledInbound(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:        "vless-disabled",
			Name:      "Disabled",
			Protocol:  repository.ProtocolVLESS,
			Transport: "tcp",
			Security:  "none",
			Host:      "disabled.example.com",
			Port:      1111,
			Enabled:   false,
		},
		{
			ID:        "vless-enabled",
			Name:      "Enabled",
			Protocol:  repository.ProtocolVLESS,
			Transport: "ws",
			Security:  "tls",
			Host:      "enabled.example.com",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"sni":"cdn.example.com","path":"/ws"}`,
		},
	}

	artifact, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err != nil {
		t.Fatalf("build artifacts: %v", err)
	}
	if !strings.Contains(artifact.AccessURI, "@enabled.example.com:443?") {
		t.Fatalf("unexpected access uri: %s", artifact.AccessURI)
	}
	if strings.Contains(artifact.AccessURI, "disabled.example.com") {
		t.Fatalf("disabled inbound leaked into uri: %s", artifact.AccessURI)
	}
	if !strings.Contains(artifact.AccessURI, "packetEncoding=xudp") {
		t.Fatalf("expected xudp packet encoding in uri: %s", artifact.AccessURI)
	}
	if !strings.Contains(artifact.AccessURI, "sni=cdn.example.com") {
		t.Fatalf("expected sni in uri: %s", artifact.AccessURI)
	}
}

func TestSingBoxAdapterBuildArtifactsRequiresEnabledInbound(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:        "vless-disabled",
			Name:      "Disabled",
			Protocol:  repository.ProtocolVLESS,
			Transport: "tcp",
			Security:  "none",
			Host:      "disabled.example.com",
			Port:      1111,
			Enabled:   false,
		},
	}

	_, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err == nil {
		t.Fatalf("expected an error when enabled inbound is missing")
	}
}

func TestSingBoxAdapterBuildArtifactsRealityDefaultsToCloudflareSNI(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:        "vless-enabled",
			Name:      "Enabled",
			Protocol:  repository.ProtocolVLESS,
			Transport: "tcp",
			Security:  "reality",
			Host:      "edge.example.com",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12"}`,
		},
	}

	artifact, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err != nil {
		t.Fatalf("build artifacts: %v", err)
	}
	if !strings.Contains(artifact.AccessURI, "sni=www.cloudflare.com") {
		t.Fatalf("expected cloudflare sni fallback, got: %s", artifact.AccessURI)
	}
}

func TestSingBoxAdapterBuildArtifactsRealityUsesDestAsSNI(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:        "vless-enabled",
			Name:      "Enabled",
			Protocol:  repository.ProtocolVLESS,
			Transport: "tcp",
			Security:  "reality",
			Host:      "edge.example.com",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.speedtest.net:443"}`,
		},
	}

	artifact, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err != nil {
		t.Fatalf("build artifacts: %v", err)
	}
	if !strings.Contains(artifact.AccessURI, "sni=www.speedtest.net") {
		t.Fatalf("expected dest sni fallback, got: %s", artifact.AccessURI)
	}
}

func TestBuildSingBoxVLESSConfigRejectsUnsupportedFlow(t *testing.T) {
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
			ParamsJSON: `{"flow":"invalid-flow","sni":"cdn.example.com","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}

	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "ok", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
			},
		},
	}

	_, err := buildSingBoxVLESSConfig(inbounds, users, "panel.example.com")
	if err == nil {
		t.Fatalf("expected flow validation error")
	}
}

func TestBuildSingBoxVLESSConfigRejectsRealityWithWSTransport(t *testing.T) {
	inbounds := []repository.Inbound{
		{
			ID:        "vless-main",
			Name:      "VLESS Main",
			Protocol:  repository.ProtocolVLESS,
			Transport: "ws",
			Security:  "reality",
			Host:      "example.com",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"flow":"xtls-rprx-vision","path":"/ws","sni":"cdn.example.com","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}

	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "ok", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
			},
		},
	}

	_, err := buildSingBoxVLESSConfig(inbounds, users, "panel.example.com")
	if err == nil {
		t.Fatalf("expected reality transport validation error")
	}
}
