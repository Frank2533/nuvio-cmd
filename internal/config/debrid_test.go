package config

import "testing"

func TestLoadDebridEmptyByDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	providers, err := LoadDebrid()
	if err != nil {
		t.Fatalf("LoadDebrid: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("LoadDebrid() on a fresh install = %+v, want empty (no default provider)", providers)
	}
}

func TestSaveAndLoadDebridRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := []DebridEntry{
		{Provider: "realdebrid", Token: "secret-token", Enabled: true},
	}
	if err := SaveDebrid(want); err != nil {
		t.Fatalf("SaveDebrid: %v", err)
	}

	got, err := LoadDebrid()
	if err != nil {
		t.Fatalf("LoadDebrid: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("LoadDebrid() = %+v, want %+v", got, want)
	}
}
