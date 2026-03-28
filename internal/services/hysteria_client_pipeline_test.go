package services

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeClientYAML(t *testing.T, raw string) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal client yaml: %v\n%s", err, raw)
	}
	return out
}

func validationHasError(v Hy2ClientValidation, expected string) bool {
	for _, err := range v.Errors {
		if strings.Contains(err, expected) {
			return true
		}
	}
	return false
}

func TestClientArtifactsPlainServerUserpassSocks5(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	profile := Hy2ClientProfile{
		Server:   "hy2.example.com:443",
		AuthType: "userpass",
		Auth:     "alice:pass12345",
		TLS:      Hy2ClientTLS{SNI: "hy2.example.com"},
		Socks5:   &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}

	artifacts, validation := manager.GenerateClientArtifacts(profile)
	if !validation.Valid {
		t.Fatalf("expected valid artifacts, got errors: %#v", validation.Errors)
	}
	if !strings.Contains(artifacts.URI, "alice:pass12345@hy2.example.com:443/") {
		t.Fatalf("unexpected URI auth/authority: %s", artifacts.URI)
	}
	if strings.Contains(artifacts.URI, "listen=") {
		t.Fatalf("URI must not contain client-local mode fields: %s", artifacts.URI)
	}

	cfg := decodeClientYAML(t, artifacts.ClientYAML)
	if _, ok := cfg["auth"]; !ok {
		t.Fatalf("expected auth in plain-server client YAML: %s", artifacts.ClientYAML)
	}
	if _, ok := cfg["socks5"]; !ok {
		t.Fatalf("expected socks5 mode in client YAML: %s", artifacts.ClientYAML)
	}
	if _, ok := cfg["http"]; ok {
		t.Fatalf("did not expect http mode in this fixture: %s", artifacts.ClientYAML)
	}
}

func TestClientArtifactsPlainServerPasswordHTTP(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	profile := Hy2ClientProfile{
		Server:   "hy2.example.com:8443",
		AuthType: "password",
		Auth:     "shared-password",
		HTTP:     &Hy2ClientHTTPMode{Listen: "127.0.0.1:8080"},
	}

	artifacts, validation := manager.GenerateClientArtifacts(profile)
	if !validation.Valid {
		t.Fatalf("expected valid artifacts, got errors: %#v", validation.Errors)
	}
	if !strings.Contains(artifacts.URI, "shared-password@hy2.example.com:8443/") {
		t.Fatalf("unexpected URI for password auth: %s", artifacts.URI)
	}

	cfg := decodeClientYAML(t, artifacts.ClientYAML)
	if _, ok := cfg["http"]; !ok {
		t.Fatalf("expected http mode in YAML: %s", artifacts.ClientYAML)
	}
	if _, ok := cfg["socks5"]; ok {
		t.Fatalf("did not expect socks5 mode in HTTP-only fixture: %s", artifacts.ClientYAML)
	}
}

func TestClientArtifactsURIServerModeOmitsDuplicatedConnectionFields(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	profile := Hy2ClientProfile{
		Server: "hysteria2://demo-user:secret@hy2.example.com:443/?insecure=1&obfs=salamander&obfs-password=obfs-secret&pinSHA256=AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899&sni=cdn.example.com",
		Socks5: &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}

	artifacts, validation := manager.GenerateClientArtifacts(profile)
	if !validation.Valid {
		t.Fatalf("expected valid URI-mode artifacts, got errors: %#v", validation.Errors)
	}
	if !strings.HasPrefix(artifacts.URI, "hysteria2://") {
		t.Fatalf("expected hysteria2 URI scheme: %s", artifacts.URI)
	}
	if !strings.HasPrefix(artifacts.URIHy2, "hy2://") {
		t.Fatalf("expected hy2 alias URI scheme: %s", artifacts.URIHy2)
	}

	cfg := decodeClientYAML(t, artifacts.ClientYAML)
	if _, ok := cfg["auth"]; ok {
		t.Fatalf("auth must be omitted when server is hysteria2:// URI: %s", artifacts.ClientYAML)
	}
	if _, ok := cfg["obfs"]; ok {
		t.Fatalf("obfs must be omitted when URI already embeds obfs values: %s", artifacts.ClientYAML)
	}
	if tls, ok := toStringAnyMap(cfg["tls"]); ok {
		if _, hasSNI := tls["sni"]; hasSNI {
			t.Fatalf("tls.sni must be omitted in URI mode: %s", artifacts.ClientYAML)
		}
		if _, hasInsecure := tls["insecure"]; hasInsecure {
			t.Fatalf("tls.insecure must be omitted in URI mode: %s", artifacts.ClientYAML)
		}
		if _, hasPin := tls["pinSHA256"]; hasPin {
			t.Fatalf("tls.pinSHA256 must be omitted in URI mode: %s", artifacts.ClientYAML)
		}
	}

	badProfile := profile
	badProfile.Auth = "duplicate-auth"
	badValidation := manager.ValidateClientProfile(badProfile)
	if badValidation.Valid {
		t.Fatalf("expected duplicate auth with URI server to be rejected")
	}
	if !validationHasError(badValidation, "must be omitted when profile.server is a hysteria2:// URI") {
		t.Fatalf("expected URI duplication validation error, got: %#v", badValidation.Errors)
	}
}

func TestClientArtifactsTLSCAAndInsecurePin(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")

	t.Run("tls-ca", func(t *testing.T) {
		profile := Hy2ClientProfile{
			Server:   "hy2.example.com:443",
			AuthType: "userpass",
			Auth:     "demo:secret123",
			TLS:      Hy2ClientTLS{SNI: "hy2.example.com", CA: "/etc/ssl/hy2-ca.crt"},
			Socks5:   &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
		}
		artifacts, validation := manager.GenerateClientArtifacts(profile)
		if !validation.Valid {
			t.Fatalf("expected valid tls-ca artifact, got: %#v", validation.Errors)
		}
		if strings.Contains(artifacts.URI, "ca=") {
			t.Fatalf("URI must not carry client-local tls.ca: %s", artifacts.URI)
		}
		if !strings.Contains(artifacts.ClientYAML, "ca: /etc/ssl/hy2-ca.crt") {
			t.Fatalf("expected tls.ca in client YAML: %s", artifacts.ClientYAML)
		}
	})

	t.Run("insecure-pin", func(t *testing.T) {
		profile := Hy2ClientProfile{
			Server:   "hy2.example.com:443",
			AuthType: "userpass",
			Auth:     "demo:secret123",
			TLS: Hy2ClientTLS{
				Insecure:  true,
				PinSHA256: "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99",
			},
			Socks5: &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
		}
		artifacts, validation := manager.GenerateClientArtifacts(profile)
		if !validation.Valid {
			t.Fatalf("expected valid insecure+pin artifact, got: %#v", validation.Errors)
		}
		if !strings.Contains(artifacts.URI, "insecure=1") {
			t.Fatalf("expected insecure=1 in URI: %s", artifacts.URI)
		}
		if !strings.Contains(artifacts.URI, "pinSHA256=aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899") {
			t.Fatalf("expected normalized pinSHA256 in URI: %s", artifacts.URI)
		}
		if !strings.Contains(artifacts.ClientYAML, "pinSHA256: AA:BB:CC:DD") {
			t.Fatalf("expected pinSHA256 in YAML: %s", artifacts.ClientYAML)
		}
	})
}

func TestClientArtifactsObfsSalamander(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	profile := Hy2ClientProfile{
		Server:   "hy2.example.com:443",
		AuthType: "userpass",
		Auth:     "demo:secret123",
		Obfs: &Hy2ClientObfs{
			Type:       "salamander",
			Salamander: &Hy2ClientSalamander{Password: "obfs-password"},
		},
		Socks5: &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}
	artifacts, validation := manager.GenerateClientArtifacts(profile)
	if !validation.Valid {
		t.Fatalf("expected valid obfs artifact, got errors: %#v", validation.Errors)
	}
	if !strings.Contains(artifacts.URI, "obfs=salamander") || !strings.Contains(artifacts.URI, "obfs-password=obfs-password") {
		t.Fatalf("expected salamander params in URI: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.ClientYAML, "obfs:") || !strings.Contains(artifacts.ClientYAML, "password: obfs-password") {
		t.Fatalf("expected obfs section in YAML: %s", artifacts.ClientYAML)
	}
}

func TestClientProfileValidationAuthAndModeRequirements(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")

	invalidUserpass := Hy2ClientProfile{
		Server:   "hy2.example.com:443",
		AuthType: "userpass",
		Auth:     "bad-format",
		Socks5:   &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}
	v := manager.ValidateClientProfile(invalidUserpass)
	if v.Valid {
		t.Fatalf("expected invalid userpass auth format")
	}
	if !validationHasError(v, "username:password") {
		t.Fatalf("expected userpass format validation error, got: %#v", v.Errors)
	}

	missingMode := Hy2ClientProfile{
		Server:   "hy2.example.com:443",
		AuthType: "password",
		Auth:     "secret123",
	}
	v = manager.ValidateClientProfile(missingMode)
	if v.Valid {
		t.Fatalf("expected missing mode validation error")
	}
	if !validationHasError(v, "at least one client mode") {
		t.Fatalf("expected at least one mode validation error, got: %#v", v.Errors)
	}
}

func TestClientArtifactsBandwidthModes(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")

	t.Run("no-bandwidth", func(t *testing.T) {
		profile := Hy2ClientProfile{
			Server:   "hy2.example.com:443",
			AuthType: "password",
			Auth:     "secret123",
			Socks5:   &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
		}
		artifacts, validation := manager.GenerateClientArtifacts(profile)
		if !validation.Valid {
			t.Fatalf("expected valid no-bandwidth artifact, got: %#v", validation.Errors)
		}
		cfg := decodeClientYAML(t, artifacts.ClientYAML)
		if _, ok := cfg["bandwidth"]; ok {
			t.Fatalf("bandwidth must not be exported when not configured: %s", artifacts.ClientYAML)
		}
	})

	t.Run("with-bandwidth", func(t *testing.T) {
		profile := Hy2ClientProfile{
			Server:   "hy2.example.com:443",
			AuthType: "password",
			Auth:     "secret123",
			Bandwidth: &Hy2ClientBandwidth{
				Up:   "20 mbps",
				Down: "100 mbps",
			},
			Socks5: &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
		}
		artifacts, validation := manager.GenerateClientArtifacts(profile)
		if !validation.Valid {
			t.Fatalf("expected valid bandwidth artifact, got: %#v", validation.Errors)
		}
		cfg := decodeClientYAML(t, artifacts.ClientYAML)
		bandwidth, ok := toStringAnyMap(cfg["bandwidth"])
		if !ok {
			t.Fatalf("expected bandwidth map in YAML: %s", artifacts.ClientYAML)
		}
		if strings.TrimSpace(toString(bandwidth["up"])) != "20 mbps" || strings.TrimSpace(toString(bandwidth["down"])) != "100 mbps" {
			t.Fatalf("unexpected bandwidth values: %#v", bandwidth)
		}
	})
}

func TestSplitServerForClientUnderstandsURIMultiPort(t *testing.T) {
	host, ports := splitServerForClient("hysteria2://demo:pass@hy2.example.com:443,5000-5002/?sni=cdn.example.com")
	if host != "hy2.example.com" {
		t.Fatalf("unexpected host: %s", host)
	}
	if ports != "443,5000-5002" {
		t.Fatalf("unexpected port union: %s", ports)
	}
}
