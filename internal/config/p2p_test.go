package config

import "testing"

func TestLoadP2PConsentDefaultsFalse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	given, err := LoadP2PConsent()
	if err != nil {
		t.Fatalf("LoadP2PConsent: %v", err)
	}
	if given {
		t.Error("LoadP2PConsent() on a fresh install = true, want false (not yet asked)")
	}
}

func TestSaveAndLoadP2PConsentRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SaveP2PConsent(true); err != nil {
		t.Fatalf("SaveP2PConsent: %v", err)
	}
	given, err := LoadP2PConsent()
	if err != nil {
		t.Fatalf("LoadP2PConsent: %v", err)
	}
	if !given {
		t.Error("LoadP2PConsent() after SaveP2PConsent(true) = false, want true")
	}
}
