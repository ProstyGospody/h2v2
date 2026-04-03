package runtime

import (
	"encoding/base64"
	"strings"
	"testing"

	"h2v2/internal/repository"
)

func TestNormalizePublicEndpointHost(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  string
	}{
		{name: "domain-with-scheme", raw: "https://Example.COM:443/path", want: "example.com"},
		{name: "loopback-ipv4", raw: "127.0.0.1", want: ""},
		{name: "loopback-ipv6", raw: "[::1]:443", want: ""},
		{name: "localhost-domain", raw: "localhost", want: ""},
		{name: "local-domain", raw: "dev.local", want: ""},
		{name: "public-ip", raw: "203.0.113.10:8443", want: "203.0.113.10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizePublicEndpointHost(tc.raw)
			if got != tc.want {
				t.Fatalf("unexpected normalized host: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestParseJSONMap(t *testing.T) {
	valid := parseJSONMap(`{"sni":"cdn.example.com","alpn":["h2","http/1.1"]}`)
	if readString(valid, "sni") != "cdn.example.com" {
		t.Fatalf("expected sni from valid json map")
	}
	alpn := readStringSlice(valid, "alpn")
	if len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("unexpected alpn parse: %+v", alpn)
	}

	invalid := parseJSONMap(`{"broken":`)
	if len(invalid) != 0 {
		t.Fatalf("expected empty map on invalid json, got %+v", invalid)
	}
}

func TestUserCredentialSelectsByProtocol(t *testing.T) {
	user := repository.UserWithCredentials{
		User: repository.User{ID: "u1", Enabled: true},
		Credentials: []repository.Credential{
			{Protocol: repository.ProtocolVLESS, Identity: "   "},
			{Protocol: repository.ProtocolHY2, Identity: "hy2-id", Secret: "hy2-secret"},
			{Protocol: repository.ProtocolVLESS, Identity: "ae299911-bf1c-45d4-a6f5-23395f8f731a"},
		},
	}

	vless, ok := userCredential(user, repository.ProtocolVLESS)
	if !ok {
		t.Fatalf("expected vless credential")
	}
	if vless.Identity != "ae299911-bf1c-45d4-a6f5-23395f8f731a" {
		t.Fatalf("unexpected vless identity: %q", vless.Identity)
	}

	hy2, ok := userCredential(user, repository.ProtocolHY2)
	if !ok {
		t.Fatalf("expected hy2 credential")
	}
	if hy2.Identity != "hy2-id" || hy2.Secret != "hy2-secret" {
		t.Fatalf("unexpected hy2 credential: %+v", hy2)
	}
}

func TestRenderVLESSClientConfigIncludesURI(t *testing.T) {
	uri := "vless://ae299911-bf1c-45d4-a6f5-23395f8f731a@example.com:443?type=tcp#demo"
	rendered := renderVLESSClientConfig(
		uri,
		"- name: demo\n  type: vless",
		map[string]any{"type": "vless"},
	)
	if !strings.Contains(rendered, uri) {
		t.Fatalf("expected rendered config to include uri: %s", rendered)
	}
}

func TestDecodeRealityKeyRejectsInvalidSize(t *testing.T) {
	_, err := decodeRealityKey("Zm9v")
	if err == nil {
		t.Fatalf("expected invalid reality key size error")
	}
}

func TestNormalizeRealityParams(t *testing.T) {
	privateKeyRaw := strings.Repeat("A", 32)
	publicKeyRaw := strings.Repeat("B", 32)
	params := map[string]any{
		"privateKey": base64.RawURLEncoding.EncodeToString([]byte(privateKeyRaw)),
		"publicKey":  base64.RawURLEncoding.EncodeToString([]byte(publicKeyRaw)),
		"sid":        "AB12",
	}

	if err := normalizeRealityParams("reality", params); err != nil {
		t.Fatalf("normalize reality params: %v", err)
	}
	if strings.TrimSpace(readString(params, "privateKey")) == "" {
		t.Fatalf("privateKey must be present")
	}
	if strings.TrimSpace(readString(params, "pbk")) == "" {
		t.Fatalf("pbk must be present")
	}
	if readString(params, "publicKey") != "" {
		t.Fatalf("legacy publicKey should be removed")
	}
	if readString(params, "sid") != "ab12" {
		t.Fatalf("sid should be normalized to lowercase hex")
	}
	if readString(params, "network") != "tcp" {
		t.Fatalf("network default should be tcp")
	}
	if readString(params, "security") != "reality" {
		t.Fatalf("security default should be reality")
	}
}

func TestNormalizeRealityParamsDerivesPublicKeyWhenMissing(t *testing.T) {
	privateKeyRaw := strings.Repeat("C", 32)
	params := map[string]any{
		"privateKey": base64.RawURLEncoding.EncodeToString([]byte(privateKeyRaw)),
	}

	if err := normalizeRealityParams("reality", params); err != nil {
		t.Fatalf("normalize reality params: %v", err)
	}
	if strings.TrimSpace(readString(params, "pbk")) == "" {
		t.Fatalf("expected derived pbk from private key")
	}
}
