package hysteria

import "testing"

func TestNormalizeUsernameLowercasesAndRejectsInvalidValues(t *testing.T) {
	username, err := NormalizeUsername(" Demo.User ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "demo.user" {
		t.Fatalf("unexpected normalized username: %s", username)
	}

	if _, err := NormalizeUsername("A"); err == nil {
		t.Fatalf("expected short username to be rejected")
	}
	if _, err := NormalizeUsername("bad value"); err == nil {
		t.Fatalf("expected username with spaces to be rejected")
	}
}

func TestValidateUserInputReportsFieldErrors(t *testing.T) {
	errs := ValidateUserInput("bad value", "short")
	if len(errs) != 2 {
		t.Fatalf("expected 2 validation errors, got %d", len(errs))
	}
	if errs[0].Field != "username" {
		t.Fatalf("unexpected first field: %s", errs[0].Field)
	}
	if errs[1].Field != "password" {
		t.Fatalf("unexpected second field: %s", errs[1].Field)
	}
}

func TestIsValidPinSHA256SupportsCommonFormats(t *testing.T) {
	if !IsValidPinSHA256("aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99") {
		t.Fatalf("expected colon-delimited hash to be valid")
	}
	if !IsValidPinSHA256("AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899") {
		t.Fatalf("expected plain hex hash to be valid")
	}
	if IsValidPinSHA256("pin-value") {
		t.Fatalf("expected non-hex hash to be rejected")
	}
}

func TestValidateClientOverridesRejectsInvalidPinHash(t *testing.T) {
	pin := "definitely-not-a-hash"
	errs := ValidateClientOverrides(&ClientOverrides{PinSHA256: &pin})
	if len(errs) == 0 {
		t.Fatalf("expected validation errors")
	}
	found := false
	for _, err := range errs {
		if err.Field == "overrides.pinSHA256" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pinSHA256 validation error, got: %#v", errs)
	}
}

