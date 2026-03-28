package handlers

import "testing"

func TestParseQRSizeClampsToSupportedRange(t *testing.T) {
	if got := parseQRSize("100", 320); got != 160 {
		t.Fatalf("expected lower clamp, got %d", got)
	}
	if got := parseQRSize("999", 320); got != 640 {
		t.Fatalf("expected upper clamp, got %d", got)
	}
	if got := parseQRSize("360", 320); got != 360 {
		t.Fatalf("expected explicit size, got %d", got)
	}
	if got := parseQRSize("broken", 320); got != 320 {
		t.Fatalf("expected fallback size, got %d", got)
	}
}
