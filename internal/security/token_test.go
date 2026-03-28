package security

import "testing"

func TestTokenGenerationAndHash(t *testing.T) {
	token, err := NewToken(24)
	if err != nil {
		t.Fatalf("NewToken failed: %v", err)
	}
	if len(token) < 24 {
		t.Fatalf("unexpected token length: %d", len(token))
	}

	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Fatalf("hash should be deterministic")
	}

	hexValue, err := RandomHex(16)
	if err != nil {
		t.Fatalf("RandomHex failed: %v", err)
	}
	if len(hexValue) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(hexValue))
	}
}

