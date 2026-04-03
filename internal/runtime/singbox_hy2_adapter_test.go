package runtime

import (
	"reflect"
	"strings"
	"testing"

	"h2v2/internal/repository"
)

func TestBuildSingBoxConfigWithStatsIncludesHY2AndVLESSUsers(t *testing.T) {
	inbounds := []repository.Inbound{
		{
			ID:         "hy2-main",
			Name:       "hy2-main",
			Protocol:   repository.ProtocolHY2,
			Transport:  "quic",
			Security:   "tls",
			Host:       "hy2.example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{"sni":"hy2.example.com","certificate_path":"/etc/ssl/hy2.crt","key_path":"/etc/ssl/hy2.key","obfs_type":"salamander","obfs_password":"obfs-secret"}`,
		},
		{
			ID:         "vless-main",
			Name:       "vless-main",
			Protocol:   repository.ProtocolVLESS,
			Transport:  "tcp",
			Security:   "reality",
			Host:       "vless.example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{"flow":"xtls-rprx-vision","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}

	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "hy2-user", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolHY2, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", Secret: "supersecret88"},
			},
		},
		{
			User: repository.User{ID: "u2", Name: "vless-user", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "ae299911-bf1c-45d4-a6f5-23395f8f731a"},
			},
		},
	}

	config, err := buildSingBoxVLESSConfigWithStats(inbounds, users, "panel.example.com", true, "127.0.0.1:10086")
	if err != nil {
		t.Fatalf("build config with stats: %v", err)
	}

	experimental, ok := config["experimental"].(map[string]any)
	if !ok {
		t.Fatalf("experimental block is missing")
	}
	v2rayAPI, ok := experimental["v2ray_api"].(map[string]any)
	if !ok {
		t.Fatalf("v2ray_api block is missing")
	}
	stats, ok := v2rayAPI["stats"].(map[string]any)
	if !ok {
		t.Fatalf("stats block is missing")
	}

	inboundTags, ok := stats["inbounds"].([]string)
	if !ok {
		t.Fatalf("stats.inbounds has invalid type: %+v", stats["inbounds"])
	}
	if !reflect.DeepEqual(inboundTags, []string{"hy2-main", "vless-main"}) {
		t.Fatalf("unexpected stats.inbounds: %+v", inboundTags)
	}

	usersList, ok := stats["users"].([]string)
	if !ok {
		t.Fatalf("stats.users has invalid type: %+v", stats["users"])
	}
	if !reflect.DeepEqual(usersList, []string{
		"2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad",
		"ae299911-bf1c-45d4-a6f5-23395f8f731a",
	}) {
		t.Fatalf("unexpected stats.users: %+v", usersList)
	}
}

func TestSingBoxHY2AdapterBuildArtifactsUsesCanonicalURIAndSubscription(t *testing.T) {
	shared := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	adapter := NewSingBoxHY2Adapter(shared)
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "demo", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolHY2, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", Secret: "supersecret88"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:         "hy2-main",
			Name:       "hy2-main",
			Protocol:   repository.ProtocolHY2,
			Transport:  "quic",
			Security:   "tls",
			Host:       "hy2.example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{"sni":"hy2.example.com","obfs_type":"salamander","obfs_password":"obfs-secret"}`,
		},
	}

	subscriptionURL := "https://panel.example.com/api/subscriptions/token-demo"
	artifact, err := adapter.BuildArtifacts(nil, user, inbounds, subscriptionURL)
	if err != nil {
		t.Fatalf("build hy2 artifacts: %v", err)
	}
	if !strings.HasPrefix(artifact.AccessURI, "hy2://") {
		t.Fatalf("expected canonical hy2 scheme, got %s", artifact.AccessURI)
	}
	if !strings.Contains(artifact.AccessURI, "supersecret88@hy2.example.com:443") {
		t.Fatalf("unexpected access uri: %s", artifact.AccessURI)
	}
	if artifact.Subscription != subscriptionURL {
		t.Fatalf("unexpected subscription url: %s", artifact.Subscription)
	}
	if !strings.Contains(artifact.Config, artifact.AccessURI) {
		t.Fatalf("expected rendered config to include canonical access uri")
	}
}

func TestMapHY2StatsIdentityUsesUUIDOnly(t *testing.T) {
	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "alpha", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolHY2, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", Secret: "secret"},
			},
		},
	}

	mapped := mapHY2StatsIdentity(users)
	if mapped["2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"] != "u1" {
		t.Fatalf("uuid mapping is missing: %+v", mapped)
	}
	if mapped["2B7EE3CD-20F0-4BD3-B9CC-10AEEB6A46AD"] != "u1" {
		t.Fatalf("uuid case-insensitive mapping is missing: %+v", mapped)
	}
	if mapped["alpha"] != "" {
		t.Fatalf("unexpected legacy name mapping: %+v", mapped)
	}
}
