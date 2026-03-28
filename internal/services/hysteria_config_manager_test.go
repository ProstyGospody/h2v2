package services

import (
	"strings"
	"testing"
)

func TestHysteriaConfigManagerValidateServerConfig(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
  email: admin@example.com
  type: http
auth:
  type: password
  password: secret-pass
obfs:
  type: salamander
  salamander:
    password: obfs-pass
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	validation := manager.Validate(cfg)
	if !validation.Valid {
		t.Fatalf("expected valid config, got errors: %#v", validation.Errors)
	}
	if validation.Summary.Listen != ":443" {
		t.Fatalf("unexpected listen summary: %s", validation.Summary.Listen)
	}
	if validation.Summary.TLSMode != "acme" {
		t.Fatalf("unexpected tls mode: %s", validation.Summary.TLSMode)
	}
	if validation.Summary.ObfsType != "salamander" {
		t.Fatalf("unexpected obfs type: %s", validation.Summary.ObfsType)
	}
}

func TestHysteriaConfigManagerApplySettingsRoundTrip(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
  email: admin@example.com
  type: http
auth:
  type: password
  password: old-secret
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	next, validation := manager.ApplySettings(cfg, Hy2Settings{
		Listen:  ":8443",
		TLSMode: "acme",
		ACME: &Hy2ServerACME{
			Domains: []string{"hy2.example.com"},
			Email:   "admin@example.com",
		},
		Auth: Hy2ServerAuth{Type: "password", Password: "new-secret"},
		Obfs: &Hy2ServerObfs{Type: "salamander", Salamander: &Hy2ServerSalamander{Password: "obfs-pass"}},
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}

	summary := manager.Parse(next)
	if summary.Listen != ":8443" {
		t.Fatalf("listen not updated: %s", summary.Listen)
	}
	if summary.ObfsType != "salamander" {
		t.Fatalf("obfs not updated: %s", summary.ObfsType)
	}
	if !strings.Contains(next, "password: new-secret") {
		t.Fatalf("auth password was not updated: %s", next)
	}
}

func TestHysteriaConfigManagerApplySettingsWithCustomQUIC(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
  email: admin@example.com
auth:
  type: password
  password: old-secret
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	next, validation := manager.ApplySettings(cfg, Hy2Settings{
		Listen:      ":443",
		TLSEnabled:  true,
		TLSMode:     "acme",
		ACME:        &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:        Hy2ServerAuth{Type: "password", Password: "new-secret"},
		QUICEnabled: true,
		QUIC: &Hy2ServerQUIC{
			MaxIdleTimeout:          "30s",
			DisablePathMTUDiscovery: true,
		},
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if !strings.Contains(next, "quic:") {
		t.Fatalf("quic section is missing: %s", next)
	}
	if !strings.Contains(next, "maxIdleTimeout: 30s") {
		t.Fatalf("quic.maxIdleTimeout is missing: %s", next)
	}
}

func TestHysteriaConfigManagerGenerateClientArtifacts(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	profile := Hy2ClientProfile{
		Name:   "demo",
		Server: "hy2.example.com:443",
		Auth:   "demo-user:supersecret88",
		TLS: Hy2ClientTLS{
			SNI:       "hy2.example.com",
			Insecure:  true,
			PinSHA256: "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99",
		},
		Obfs: &Hy2ClientObfs{
			Type:       "salamander",
			Salamander: &Hy2ClientSalamander{Password: "obfs-pass"},
		},
		QUIC: &Hy2ClientQUIC{
			MaxIdleTimeout:          "30s",
			DisablePathMTUDiscovery: true,
		},
		Socks5: &Hy2ClientSocks5Mode{Listen: "127.0.0.1:1080"},
	}

	artifacts, validation := manager.GenerateClientArtifacts(profile)
	if !validation.Valid {
		t.Fatalf("profile validation failed: %#v", validation.Errors)
	}
	if !strings.HasPrefix(artifacts.URI, "hysteria2://") {
		t.Fatalf("unexpected URI scheme: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.URI, "demo-user:supersecret88@hy2.example.com:443/") {
		t.Fatalf("expected userpass auth and authority in URI: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.URI, "obfs=salamander") {
		t.Fatalf("obfs type is missing in URI: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.URI, "obfs-password=obfs-pass") {
		t.Fatalf("obfs password is missing in URI: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.URI, "pinSHA256=aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899") {
		t.Fatalf("pinSHA256 is missing or unnormalized in URI: %s", artifacts.URI)
	}

	if strings.Contains(artifacts.URI, "maxIdleTimeout") {
		t.Fatalf("URI must not include QUIC-only params: %s", artifacts.URI)
	}
	if !strings.Contains(artifacts.ClientYAML, "quic:") {
		t.Fatalf("client YAML does not contain quic section: %s", artifacts.ClientYAML)
	}
	if !strings.Contains(artifacts.ClientYAML, "maxIdleTimeout: 30s") {
		t.Fatalf("client YAML does not contain maxIdleTimeout: %s", artifacts.ClientYAML)
	}
	if !strings.Contains(artifacts.ClientYAML, "socks5:") {
		t.Fatalf("client YAML does not contain socks5 mode: %s", artifacts.ClientYAML)
	}
	if !strings.Contains(artifacts.ClientYAML, "pinSHA256: AA:BB:CC:DD") {
		t.Fatalf("client YAML must contain scalar pinSHA256 value: %s", artifacts.ClientYAML)
	}
}

func TestDefaultClientProfileFromSettingsUsesACMEDomainWhenFallbackWildcard(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")

	settings := Hy2Settings{
		Listen:            "0.0.0.0:443",
		TLSEnabled:        true,
		TLSMode:           "acme",
		ClientTLSInsecure: true,
		ACME:              &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:              Hy2ServerAuth{Type: "password", Password: "secret"},
	}

	profile := manager.DefaultClientProfileFromSettings(settings, "0.0.0.0", 443, "token")
	if !strings.HasPrefix(profile.Server, "hy2.example.com:") {
		t.Fatalf("expected ACME domain fallback for server host, got: %s", profile.Server)
	}
	if profile.TLS.SNI != "hy2.example.com" {
		t.Fatalf("expected ACME domain SNI, got: %s", profile.TLS.SNI)
	}
	if !profile.TLS.Insecure {
		t.Fatalf("expected default client tls.insecure to inherit server setting")
	}
}

func TestValidateSettingsAllowsPortUnionListen(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	validation := manager.Validate(`listen: :443,8443-8450
acme:
  domains:
    - hy2.example.com
auth:
  type: userpass
  userpass: {}`)
	if !validation.Valid {
		t.Fatalf("expected port-union listen to be valid, got errors: %#v", validation.Errors)
	}
}

func TestValidateSettingsAllowsBarePortListen(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	validation := manager.Validate(`listen: 443
acme:
  domains:
    - hy2.example.com
auth:
  type: userpass
  userpass: {}`)
	if !validation.Valid {
		t.Fatalf("expected bare listen port to be valid, got errors: %#v", validation.Errors)
	}
	if validation.Summary.Listen != ":443" {
		t.Fatalf("expected normalized listen :443, got: %s", validation.Summary.Listen)
	}
}

func TestValidateSettingsSupportsCommandAuth(t *testing.T) {
	manager := NewHysteriaConfigManager("/tmp/unused")
	validation := manager.Validate(`listen: :443
acme:
  domains:
    - hy2.example.com
auth:
  type: command
  command: /usr/local/bin/hy2-auth`)
	if !validation.Valid {
		t.Fatalf("expected command auth to be valid, got errors: %#v", validation.Errors)
	}
}

func TestHysteriaConfigManagerApplySettingsWithBandwidthAndUDP(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
auth:
  type: password
  password: old-secret
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	next, validation := manager.ApplySettings(cfg, Hy2Settings{
		Listen:                ":443",
		TLSEnabled:            true,
		TLSMode:               "acme",
		ACME:                  &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:                  Hy2ServerAuth{Type: "password", Password: "new-secret"},
		Bandwidth:             &Hy2ServerBandwidth{Up: "100", Down: "200"},
		IgnoreClientBandwidth: true,
		DisableUDP:            false,
		UDPIdleTimeout:        "90s",
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if !strings.Contains(next, "bandwidth:") {
		t.Fatalf("bandwidth section is missing: %s", next)
	}
	if !strings.Contains(next, "up: 100 mbps") || !strings.Contains(next, "down: 200 mbps") {
		t.Fatalf("bandwidth values are missing: %s", next)
	}
	if !strings.Contains(next, "ignoreClientBandwidth: true") {
		t.Fatalf("ignoreClientBandwidth is missing: %s", next)
	}
	if !strings.Contains(next, "udpIdleTimeout: 90s") {
		t.Fatalf("udpIdleTimeout is missing: %s", next)
	}
}

func TestHysteriaConfigManagerApplySettingsClientTLSInsecureFlag(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
auth:
  type: password
  password: old-secret
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	withInsecure, validation := manager.ApplySettings(cfg, Hy2Settings{
		Listen:            ":443",
		TLSEnabled:        true,
		TLSMode:           "acme",
		ClientTLSInsecure: true,
		ACME:              &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:              Hy2ServerAuth{Type: "password", Password: "new-secret"},
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if !strings.Contains(withInsecure, "clientTLSInsecure: true") {
		t.Fatalf("expected clientTLSInsecure in config: %s", withInsecure)
	}

	withoutInsecure, validation := manager.ApplySettings(withInsecure, Hy2Settings{
		Listen:            ":443",
		TLSEnabled:        true,
		TLSMode:           "acme",
		ClientTLSInsecure: false,
		ACME:              &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:              Hy2ServerAuth{Type: "password", Password: "new-secret"},
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if strings.Contains(withoutInsecure, "clientTLSInsecure: true") {
		t.Fatalf("expected clientTLSInsecure to be removed when disabled: %s", withoutInsecure)
	}
}

func TestHysteriaConfigManagerApplySettingsSpeedTestFlag(t *testing.T) {
	cfg := `listen: :443
acme:
  domains:
    - hy2.example.com
auth:
  type: password
  password: old-secret
`

	manager := NewHysteriaConfigManager("/tmp/unused")
	withSpeedTest, validation := manager.ApplySettings(cfg, Hy2Settings{
		Listen:      ":443",
		TLSEnabled:  true,
		TLSMode:     "acme",
		ACME:        &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:        Hy2ServerAuth{Type: "password", Password: "new-secret"},
		SpeedTest:   true,
		QUICEnabled: false,
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if !strings.Contains(withSpeedTest, "speedTest: true") {
		t.Fatalf("expected speedTest in config: %s", withSpeedTest)
	}

	withoutSpeedTest, validation := manager.ApplySettings(withSpeedTest, Hy2Settings{
		Listen:      ":443",
		TLSEnabled:  true,
		TLSMode:     "acme",
		ACME:        &Hy2ServerACME{Domains: []string{"hy2.example.com"}, Email: "admin@example.com"},
		Auth:        Hy2ServerAuth{Type: "password", Password: "new-secret"},
		SpeedTest:   false,
		QUICEnabled: false,
	})
	if !validation.Valid {
		t.Fatalf("apply validation failed: %#v", validation.Errors)
	}
	if strings.Contains(withoutSpeedTest, "speedTest: true") {
		t.Fatalf("expected speedTest to be removed when disabled: %s", withoutSpeedTest)
	}
}
