package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DebridEntry is one configured debrid provider account.
type DebridEntry struct {
	Provider string `json:"provider"` // one of debrid.KnownProviders()
	Token    string `json:"token"`
	Enabled  bool   `json:"enabled"`
}

type debridFile struct {
	Providers []DebridEntry `json:"providers"`
}

func debridPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "debrid.json"), nil
}

// LoadDebrid reads the configured debrid providers. Unlike addons, there's
// no sensible default here — a fresh install starts with none configured.
func LoadDebrid() ([]DebridEntry, error) {
	path, err := debridPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var f debridFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Providers, nil
}

// SaveDebrid persists the configured debrid providers.
func SaveDebrid(providers []DebridEntry) error {
	path, err := debridPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(debridFile{Providers: providers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) // contains API tokens
}
