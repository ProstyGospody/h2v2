package security

import "testing"

func TestHashAndComparePassword(t *testing.T) {
	password := "Str0ngPassword!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == password {
		t.Fatalf("hash should not equal password")
	}
	if err := ComparePassword(hash, password); err != nil {
		t.Fatalf("ComparePassword failed: %v", err)
	}
	if err := ComparePassword(hash, "wrong"); err == nil {
		t.Fatalf("expected compare failure for wrong password")
	}
}

