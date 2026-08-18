package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type p2pConsentFile struct {
	ConsentGiven bool `json:"consentGiven"`
}

func p2pConsentPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "p2p.json"), nil
}

// LoadP2PConsent reports whether the user has previously agreed to Nuvio
// CMD's P2P streaming disclosure. Defaults to false (not yet asked/agreed).
func LoadP2PConsent() (bool, error) {
	path, err := p2pConsentPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var f p2pConsentFile
	if err := json.Unmarshal(data, &f); err != nil {
		return false, err
	}
	return f.ConsentGiven, nil
}

// SaveP2PConsent persists the user's agreement so they aren't re-prompted
// every session.
func SaveP2PConsent(given bool) error {
	path, err := p2pConsentPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(p2pConsentFile{ConsentGiven: given}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
