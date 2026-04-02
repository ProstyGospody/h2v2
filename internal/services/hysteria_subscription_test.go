package services

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestSubscriptionTokenBuildAndVerify(t *testing.T) {
	token, err := buildSubscriptionToken("panel-secret", "user-123")
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	subject, err := parseSubscriptionTokenSubject(token)
	if err != nil {
		t.Fatalf("parse token subject: %v", err)
	}
	if subject != "user-123" {
		t.Fatalf("unexpected subject: %s", subject)
	}

	if ok := verifySubscriptionToken("panel-secret", token, "user-123", time.Time{}); !ok {
		t.Fatalf("expected token to verify")
	}
}

func TestSubscriptionTokenRejectsWrongSecretOrSubject(t *testing.T) {
	token, err := buildSubscriptionToken("panel-secret", "user-123")
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	if ok := verifySubscriptionToken("other-secret", token, "user-123", time.Time{}); ok {
		t.Fatalf("expected token verification to fail with wrong secret")
	}
	if ok := verifySubscriptionToken("panel-secret", token, "user-456", time.Time{}); ok {
		t.Fatalf("expected token verification to fail with wrong subject")
	}
}

func TestSubscriptionTokenAcceptsLegacySignature(t *testing.T) {
	updatedAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	payload := base64.RawURLEncoding.EncodeToString([]byte("user-123"))
	legacySig := computeLegacySubscriptionSignature("panel-secret", "user-123", updatedAt)
	token := payload + "." + base64.RawURLEncoding.EncodeToString(legacySig)

	if ok := verifySubscriptionToken("panel-secret", token, "user-123", updatedAt); !ok {
		t.Fatalf("expected legacy token to verify")
	}
}
