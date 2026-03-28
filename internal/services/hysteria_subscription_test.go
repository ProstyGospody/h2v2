package services

import (
	"testing"
	"time"
)

func TestSubscriptionTokenBuildAndVerify(t *testing.T) {
	updatedAt := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	token, err := buildSubscriptionToken("panel-secret", "user-123", updatedAt)
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

	if ok := verifySubscriptionToken("panel-secret", token, "user-123", updatedAt); !ok {
		t.Fatalf("expected token to verify")
	}
}

func TestSubscriptionTokenRejectsStaleVersion(t *testing.T) {
	issuedAt := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	token, err := buildSubscriptionToken("panel-secret", "user-123", issuedAt)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	rotatedAt := issuedAt.Add(10 * time.Minute)
	if ok := verifySubscriptionToken("panel-secret", token, "user-123", rotatedAt); ok {
		t.Fatalf("expected token verification to fail after updated_at rotation")
	}
}
