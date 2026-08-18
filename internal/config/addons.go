// Package config manages Nuvio CMD's on-disk configuration (currently just
// the installed addon list) under the OS config directory.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AddonEntry is one addon a user has installed, identified by its manifest
// URL (the Stremio-protocol convention).
type AddonEntry struct {
	ManifestURL string `json:"manifestUrl"`
	Enabled     bool   `json:"enabled"`
}

type addonsFile struct {
	Addons []AddonEntry `json:"addons"`
}

// defaultAddons seeds a fresh install with the official Cinemeta catalog
// addon, so Browse has something to show without any manual setup.
func defaultAddons() []AddonEntry {
	return []AddonEntry{
		{ManifestURL: "https://v3-cinemeta.strem.io/manifest.json", Enabled: true},
	}
}

// Dir returns Nuvio CMD's config directory, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "nuvio-cmd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func addonsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "addons.json"), nil
}

// LoadAddons reads the installed-addon list, seeding and persisting the
// defaults on first run.
func LoadAddons() ([]AddonEntry, error) {
	path, err := addonsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		defaults := defaultAddons()
		if err := SaveAddons(defaults); err != nil {
			return nil, err
		}
		return defaults, nil
	}
	if err != nil {
		return nil, err
	}

	var f addonsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Addons, nil
}

// SaveAddons persists the installed-addon list.
func SaveAddons(addons []AddonEntry) error {
	path, err := addonsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(addonsFile{Addons: addons}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
