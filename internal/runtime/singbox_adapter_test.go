package runtime

import (
	"reflect"
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
	if !strings.Contains(artifact.AccessURI, "#demo") {
		t.Fatalf("expected profile name fragment in uri: %s", artifact.AccessURI)
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

func TestSingBoxAdapterBuildArtifactsRejectsLoopbackHostWithoutPublicFallback(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "")
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
			Host:      "127.0.0.1",
			Port:      443,
			Enabled:   true,
			ParamsJSON: `{"privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}

	_, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err == nil {
		t.Fatalf("expected public endpoint host validation error")
	}
}

func TestBuildSingBoxVLESSConfigWithStatsIncludesUsersAndInbounds(t *testing.T) {
	inbounds := []repository.Inbound{
		{
			ID:         "vless-main",
			Name:       "vless-main",
			Protocol:   repository.ProtocolVLESS,
			Transport:  "tcp",
			Security:   "reality",
			Host:       "example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{"flow":"xtls-rprx-vision","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}
	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "alpha", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
			},
		},
		{
			User: repository.User{ID: "u2", Name: "beta", Enabled: false},
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
		t.Fatalf("experimental block is missing: %+v", config)
	}
	v2rayAPI, ok := experimental["v2ray_api"].(map[string]any)
	if !ok {
		t.Fatalf("v2ray_api block is missing: %+v", experimental)
	}
	if v2rayAPI["listen"] != "127.0.0.1:10086" {
		t.Fatalf("unexpected stats listen endpoint: %+v", v2rayAPI["listen"])
	}
	stats, ok := v2rayAPI["stats"].(map[string]any)
	if !ok {
		t.Fatalf("stats block is missing: %+v", v2rayAPI)
	}
	enabled, ok := stats["enabled"].(bool)
	if !ok || !enabled {
		t.Fatalf("expected stats.enabled=true, got %+v", stats["enabled"])
	}
	inboundTags, ok := stats["inbounds"].([]string)
	if !ok {
		t.Fatalf("stats.inbounds has invalid type: %+v", stats["inbounds"])
	}
	if !reflect.DeepEqual(inboundTags, []string{"vless-main"}) {
		t.Fatalf("unexpected stats.inbounds: %+v", inboundTags)
	}
	usersList, ok := stats["users"].([]string)
	if !ok {
		t.Fatalf("stats.users has invalid type: %+v", stats["users"])
	}
	if !reflect.DeepEqual(usersList, []string{"2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", "alpha"}) {
		t.Fatalf("unexpected stats.users: %+v", usersList)
	}
}

func TestParseSingBoxUserTrafficName(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantUser  string
		wantDir   string
		wantValid bool
	}{
		{
			name:      "uplink",
			raw:       "user>>>demo>>>traffic>>>uplink",
			wantUser:  "demo",
			wantDir:   "uplink",
			wantValid: true,
		},
		{
			name:      "downlink",
			raw:       "user>>>demo>>>traffic>>>downlink",
			wantUser:  "demo",
			wantDir:   "downlink",
			wantValid: true,
		},
		{
			name:      "invalid-prefix",
			raw:       "inbound>>>demo>>>traffic>>>uplink",
			wantValid: false,
		},
		{
			name:      "invalid-direction",
			raw:       "user>>>demo>>>traffic>>>unknown",
			wantValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotUser, gotDir, gotValid := parseSingBoxUserTrafficName(tc.raw)
			if gotValid != tc.wantValid {
				t.Fatalf("valid mismatch: want=%v got=%v", tc.wantValid, gotValid)
			}
			if !tc.wantValid {
				return
			}
			if gotUser != tc.wantUser || gotDir != tc.wantDir {
				t.Fatalf("unexpected parse result: user=%q dir=%q", gotUser, gotDir)
			}
		})
	}
}

func TestParseSingBoxUserOnlineName(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantUser  string
		wantValid bool
	}{
		{
			name:      "online",
			raw:       "user>>>demo>>>online",
			wantUser:  "demo",
			wantValid: true,
		},
		{
			name:      "connections",
			raw:       "user>>>demo>>>connections",
			wantUser:  "demo",
			wantValid: true,
		},
		{
			name:      "traffic-not-online",
			raw:       "user>>>demo>>>traffic>>>uplink",
			wantValid: false,
		},
		{
			name:      "unknown-suffix",
			raw:       "user>>>demo>>>latency",
			wantValid: false,
		},
		{
			name:      "invalid-prefix",
			raw:       "inbound>>>demo>>>online",
			wantValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotUser, gotValid := parseSingBoxUserOnlineName(tc.raw)
			if gotValid != tc.wantValid {
				t.Fatalf("valid mismatch: want=%v got=%v", tc.wantValid, gotValid)
			}
			if !tc.wantValid {
				return
			}
			if gotUser != tc.wantUser {
				t.Fatalf("unexpected user: %q", gotUser)
			}
		})
	}
}

func TestUnmarshalSingBoxQueryStatsResponse(t *testing.T) {
	encodeStat := func(name string, value int64) []byte {
		stat := make([]byte, 0, 64)
		stat = appendProtoStringField(stat, 1, name)
		stat = appendProtoTag(stat, 2, 0)
		stat = appendProtoVarint(stat, uint64(value))
		return stat
	}

	payload := make([]byte, 0, 256)
	for _, item := range []struct {
		name  string
		value int64
	}{
		{name: "user>>>alpha>>>traffic>>>uplink", value: 100},
		{name: "user>>>alpha>>>traffic>>>downlink", value: 200},
	} {
		encoded := encodeStat(item.name, item.value)
		payload = appendProtoTag(payload, 1, 2)
		payload = appendProtoVarint(payload, uint64(len(encoded)))
		payload = append(payload, encoded...)
	}

	stats, err := unmarshalSingBoxQueryStatsResponse(payload)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("unexpected stats length: %d", len(stats))
	}
	if stats[0].Name != "user>>>alpha>>>traffic>>>uplink" || stats[0].Value != 100 {
		t.Fatalf("unexpected first stat: %+v", stats[0])
	}
	if stats[1].Name != "user>>>alpha>>>traffic>>>downlink" || stats[1].Value != 200 {
		t.Fatalf("unexpected second stat: %+v", stats[1])
	}
}

func TestBuildSingBoxVLESSConfigWithStatsIncludesUserNameAndUUID(t *testing.T) {
	inbounds := []repository.Inbound{
		{
			ID:         "vless-main",
			Name:       "vless-main",
			Protocol:   repository.ProtocolVLESS,
			Transport:  "tcp",
			Security:   "reality",
			Host:       "example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{"flow":"xtls-rprx-vision","privateKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","pbk":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","sid":"ab12","dest":"www.cloudflare.com:443"}`,
		},
	}
	users := []repository.UserWithCredentials{
		{
			User: repository.User{ID: "u1", Name: "alpha", Enabled: true},
			Credentials: []repository.Credential{
				{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
			},
		},
	}

	config, err := buildSingBoxVLESSConfigWithStats(inbounds, users, "panel.example.com", true, "127.0.0.1:10086")
	if err != nil {
		t.Fatalf("build config with stats: %v", err)
	}

	experimental := config["experimental"].(map[string]any)
	v2rayAPI := experimental["v2ray_api"].(map[string]any)
	stats := v2rayAPI["stats"].(map[string]any)
	usersList := stats["users"].([]string)
	if len(usersList) != 2 {
		t.Fatalf("expected 2 users in stats.users, got %d (%+v)", len(usersList), usersList)
	}
	if !reflect.DeepEqual(usersList, []string{"2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad", "alpha"}) {
		t.Fatalf("unexpected stats.users: %+v", usersList)
	}
}

func TestSingBoxAdapterBuildArtifactsUsesUUIDAsFragmentWhenNameIsEmpty(t *testing.T) {
	adapter := NewSingBoxAdapter("", "", nil, "", "panel.example.com")
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Name: "", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad"},
		},
	}
	inbounds := []repository.Inbound{
		{
			ID:         "vless-enabled",
			Name:       "Enabled",
			Protocol:   repository.ProtocolVLESS,
			Transport:  "tcp",
			Security:   "none",
			Host:       "enabled.example.com",
			Port:       443,
			Enabled:    true,
			ParamsJSON: `{}`,
		},
	}

	artifact, err := adapter.BuildArtifacts(nil, user, inbounds, "https://sub.example.com/api/subscriptions/token")
	if err != nil {
		t.Fatalf("build artifacts: %v", err)
	}
	if !strings.Contains(artifact.AccessURI, "#2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad") {
		t.Fatalf("expected uuid fragment fallback in uri: %s", artifact.AccessURI)
	}
}
