package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

// newTestStore opens an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}


func newTestUser(t *testing.T, store *Store, username string) User {
	t.Helper()
	user, err := store.UpsertUser(context.Background(), User{
		Username: username,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("newTestUser %q: %v", username, err)
	}
	return user
}

// --- normalizeClientEndpointHost ---

func TestNormalizeClientEndpointHost(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"  example.com  ", "example.com"},
		{"https://example.com", "example.com"},
		{"https://example.com:443/path?q=1", "example.com"},
		{"example.com:8080", "example.com"},
		{"[::1]", ""},
		{"::1", ""},
		{"127.0.0.1", ""},
		{"localhost", ""},
		{"0.0.0.0", ""},
		{"::", ""},
		{"", ""},
		{"192.168.1.1", "192.168.1.1"},
		{"10.0.0.1", "10.0.0.1"},
	}
	for _, tc := range cases {
		got := normalizeClientEndpointHost(tc.input)
		if got != tc.want {
			t.Errorf("normalizeClientEndpointHost(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- sanitizeVLESSUUID ---

func TestSanitizeVLESSUUID(t *testing.T) {
	t.Run("valid uuid", func(t *testing.T) {
		id := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		got, err := sanitizeVLESSUUID(id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != strings.ToLower(id) {
			t.Errorf("got %q, want %q", got, strings.ToLower(id))
		}
	})

	t.Run("empty generates uuid", func(t *testing.T) {
		got, err := sanitizeVLESSUUID("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Error("expected non-empty uuid for empty input")
		}
		// Should be a valid UUID format.
		if _, err2 := sanitizeVLESSUUID(got); err2 != nil {
			t.Errorf("generated uuid is invalid: %v", err2)
		}
	})

	t.Run("invalid returns error", func(t *testing.T) {
		if _, err := sanitizeVLESSUUID("not-a-uuid"); err == nil {
			t.Error("expected error for invalid uuid, got nil")
		}
	})
}

// --- checkUserActive ---

func TestCheckUserActive(t *testing.T) {
	svc := &Service{}

	user := User{Enabled: true}
	access := UserAccess{Enabled: true}

	t.Run("active user and access", func(t *testing.T) {
		if !svc.checkUserActive(user, access) {
			t.Error("expected active")
		}
	})

	t.Run("disabled user", func(t *testing.T) {
		u := user
		u.Enabled = false
		if svc.checkUserActive(u, access) {
			t.Error("expected inactive for disabled user")
		}
	})

	t.Run("disabled access", func(t *testing.T) {
		a := access
		a.Enabled = false
		if svc.checkUserActive(user, a) {
			t.Error("expected inactive for disabled access")
		}
	})

	t.Run("expired user", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		u := user
		u.ExpireAt = &past
		if svc.checkUserActive(u, access) {
			t.Error("expected inactive for expired user")
		}
	})

	t.Run("not yet expired", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		u := user
		u.ExpireAt = &future
		if !svc.checkUserActive(u, access) {
			t.Error("expected active for future expiry")
		}
	})

	t.Run("access expire override overrides user", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		u := user
		u.ExpireAt = &future

		past := time.Now().Add(-time.Hour)
		a := access
		a.ExpireAtOverride = &past
		if svc.checkUserActive(u, a) {
			t.Error("expected inactive when access override is past")
		}
	})

	t.Run("traffic limit exceeded", func(t *testing.T) {
		u := user
		u.TrafficLimitBytes = 100
		u.TrafficUsedUpBytes = 60
		u.TrafficUsedDownBytes = 50 // total 110 > 100
		if svc.checkUserActive(u, access) {
			t.Error("expected inactive when traffic limit exceeded")
		}
	})

	t.Run("traffic limit not exceeded", func(t *testing.T) {
		u := user
		u.TrafficLimitBytes = 100
		u.TrafficUsedUpBytes = 40
		u.TrafficUsedDownBytes = 50 // total 90 < 100
		if !svc.checkUserActive(u, access) {
			t.Error("expected active when traffic under limit")
		}
	})

	t.Run("traffic override overrides user limit", func(t *testing.T) {
		u := user
		u.TrafficLimitBytes = 1000 // plenty
		u.TrafficUsedUpBytes = 60
		u.TrafficUsedDownBytes = 60 // total 120

		override := int64(100) // lower limit
		a := access
		a.TrafficLimitBytesOverride = &override
		if svc.checkUserActive(u, a) {
			t.Error("expected inactive when access override limit exceeded")
		}
	})
}

// --- Store CRUD: User ---

func TestStoreUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	t.Run("upsert and get", func(t *testing.T) {
		user, err := store.UpsertUser(ctx, User{Username: "alice", Enabled: true})
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if user.ID == "" {
			t.Error("expected non-empty ID")
		}
		got, err := store.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Username != "alice" {
			t.Errorf("username: got %q, want %q", got.Username, "alice")
		}
	})

	t.Run("duplicate username returns conflict", func(t *testing.T) {
		_, err := store.UpsertUser(ctx, User{Username: "bob", Enabled: true})
		if err != nil {
			t.Fatalf("first upsert: %v", err)
		}
		// Try to create another user with the same username.
		_, err = store.UpsertUser(ctx, User{Username: "bob", Enabled: true})
		if err == nil {
			t.Error("expected conflict error for duplicate username")
		}
	})

	t.Run("empty username returns error", func(t *testing.T) {
		if _, err := store.UpsertUser(ctx, User{Username: ""}); err == nil {
			t.Error("expected error for empty username")
		}
	})

	t.Run("get not found", func(t *testing.T) {
		if _, err := store.GetUser(ctx, "does-not-exist"); !IsNotFound(err) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		user, _ := store.UpsertUser(ctx, User{Username: "charlie", Enabled: true})
		if err := store.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := store.GetUser(ctx, user.ID); !IsNotFound(err) {
			t.Error("expected ErrNotFound after delete")
		}
	})
}

// --- Store: Subscription tokens ---

func TestSubscriptionToken(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	user := newTestUser(t, store, "dave")
	sub, err := store.EnsureSubscriptionForUser(ctx, user.ID, "default")
	if err != nil {
		t.Fatalf("ensure subscription: %v", err)
	}

	t.Run("issue token", func(t *testing.T) {
		issued, err := store.IssueSubscriptionToken(ctx, sub.ID, nil)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if issued.PlaintextToken == "" {
			t.Error("expected non-empty plaintext token")
		}
		if issued.Token.TokenPrefix == "" {
			t.Error("expected non-empty prefix")
		}
		// Token prefix must match the start of the plaintext token.
		if !strings.HasPrefix(issued.PlaintextToken, issued.Token.TokenPrefix) {
			t.Errorf("prefix %q not found at start of token %q", issued.Token.TokenPrefix, issued.PlaintextToken)
		}
	})

	t.Run("resolve token", func(t *testing.T) {
		issued, _ := store.IssueSubscriptionToken(ctx, sub.ID, nil)
		tc, err := store.ResolveSubscriptionToken(ctx, issued.PlaintextToken, "1.2.3.4")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if tc.User.ID != user.ID {
			t.Errorf("user id: got %q, want %q", tc.User.ID, user.ID)
		}
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		if _, err := store.ResolveSubscriptionToken(ctx, "bad-token", "1.2.3.4"); err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("rotate revokes old token", func(t *testing.T) {
		old, _ := store.IssueSubscriptionToken(ctx, sub.ID, nil)
		rotated, err := store.RotateSubscriptionToken(ctx, sub.ID, nil)
		if err != nil {
			t.Fatalf("rotate: %v", err)
		}
		// Old token should no longer resolve.
		if _, err := store.ResolveSubscriptionToken(ctx, old.PlaintextToken, "1.2.3.4"); err == nil {
			t.Error("expected old token to be invalid after rotation")
		}
		// New token should resolve.
		if _, err := store.ResolveSubscriptionToken(ctx, rotated.PlaintextToken, "1.2.3.4"); err != nil {
			t.Errorf("new token failed to resolve: %v", err)
		}
	})
}

// --- Store: AllowSubscriptionRateHit ---

func TestAllowSubscriptionRateHit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	key := "1.2.3.4|sub_abc123"
	limit := 5
	window := time.Minute

	// First <limit> requests should be allowed.
	for i := 0; i < limit; i++ {
		allowed, err := store.AllowSubscriptionRateHit(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("hit %d: %v", i, err)
		}
		if !allowed {
			t.Errorf("hit %d should be allowed, got denied", i)
		}
	}

	// The next request must be denied.
	allowed, err := store.AllowSubscriptionRateHit(ctx, key, limit, window)
	if err != nil {
		t.Fatalf("over-limit hit: %v", err)
	}
	if allowed {
		t.Error("request beyond limit should be denied")
	}
}

// --- buildVLESSClientArtifacts ---

func TestBuildVLESSClientArtifacts(t *testing.T) {
	user := User{ID: "u1", Username: "alice"}
	access := UserAccess{VLESSUUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"}
	inbound := Inbound{
		Tag:        "vless-in",
		ListenPort: 443,
		Protocol:   InboundProtocolVLESS,
		VLESS: &VLESSInboundSettings{
			TransportType:  "tcp",
			RealityEnabled: false,
			TLSEnabled:     false,
		},
	}

	uri, outbound, err := buildVLESSClientArtifacts(user, access, inbound, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(uri, "vless://") {
		t.Errorf("uri should start with vless://, got %q", uri)
	}
	if !strings.Contains(uri, "example.com:443") {
		t.Errorf("uri should contain host:port, got %q", uri)
	}
	if outbound["type"] != "vless" {
		t.Errorf("outbound type: got %v, want vless", outbound["type"])
	}
}

// --- buildHysteria2ClientArtifacts ---

func TestBuildHysteria2ClientArtifacts(t *testing.T) {
	user := User{ID: "u1", Username: "bob"}
	access := UserAccess{Hysteria2Password: "bob:secretpass"}
	inbound := Inbound{
		Tag:        "hy2-in",
		ListenPort: 8443,
		Protocol:   InboundProtocolHysteria2,
		Hysteria2: &Hysteria2InboundSettings{
			TLSEnabled: true,
		},
	}

	uri, outbound, err := buildHysteria2ClientArtifacts(user, access, inbound, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(uri, "hysteria2://") {
		t.Errorf("uri should start with hysteria2://, got %q", uri)
	}
	if !strings.Contains(uri, "example.com:8443") {
		t.Errorf("uri should contain host:port, got %q", uri)
	}
	if outbound["type"] != "hysteria2" {
		t.Errorf("outbound type: got %v, want hysteria2", outbound["type"])
	}
}

// --- tokenPrefix ---

func TestTokenPrefix(t *testing.T) {
	long := "sub_abcdefghijklmnopqrstuvwxyz"
	prefix := tokenPrefix(long)
	if len(prefix) != 12 {
		t.Errorf("prefix length: got %d, want 12", len(prefix))
	}
	if prefix != long[:12] {
		t.Errorf("prefix: got %q, want %q", prefix, long[:12])
	}

	short := "sub_abc"
	if tokenPrefix(short) != short {
		t.Errorf("short token prefix should be the token itself")
	}
}

// --- tokenHash ---

func TestTokenHash(t *testing.T) {
	salt := "deadbeef"
	token := "sub_mysecrettoken"

	h1 := tokenHash(salt, token)
	h2 := tokenHash(salt, token)

	if h1 != h2 {
		t.Error("same inputs should produce same hash")
	}

	h3 := tokenHash("othersalt", token)
	if h1 == h3 {
		t.Error("different salt should produce different hash")
	}

	if !hashEqual(h1, h2) {
		t.Error("hashEqual should return true for equal hashes")
	}
	if hashEqual(h1, h3) {
		t.Error("hashEqual should return false for different hashes")
	}
}
