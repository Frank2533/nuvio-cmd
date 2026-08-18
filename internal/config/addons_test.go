package config

import "testing"

func TestLoadAddonsSeedsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	addons, err := LoadAddons()
	if err != nil {
		t.Fatalf("LoadAddons: %v", err)
	}
	if len(addons) != 1 || addons[0].ManifestURL == "" {
		t.Fatalf("LoadAddons() = %+v, want one seeded default addon", addons)
	}

	// A second load should read back what was persisted, not reseed.
	addons2, err := LoadAddons()
	if err != nil {
		t.Fatalf("LoadAddons (second read): %v", err)
	}
	if len(addons2) != 1 || addons2[0].ManifestURL != addons[0].ManifestURL {
		t.Fatalf("second LoadAddons() = %+v, want same as first %+v", addons2, addons)
	}
}

func TestSaveAndLoadAddonsRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := []AddonEntry{
		{ManifestURL: "https://example.com/manifest.json", Enabled: true},
		{ManifestURL: "https://example.org/manifest.json", Enabled: false},
	}
	if err := SaveAddons(want); err != nil {
		t.Fatalf("SaveAddons: %v", err)
	}

	got, err := LoadAddons()
	if err != nil {
		t.Fatalf("LoadAddons: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("LoadAddons() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("addon[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
