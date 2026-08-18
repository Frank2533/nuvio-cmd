package debrid

import (
	"context"
	"errors"
	"fmt"
)

// Known provider ids, used in config persistence and to construct the
// matching Provider implementation.
const (
	ProviderRealDebrid = "realdebrid"
	ProviderAllDebrid  = "alldebrid"
	ProviderPremiumize = "premiumize"
	ProviderTorBox     = "torbox"
)

// KnownProviders lists every provider id Nuvio CMD supports, in the fixed
// order they're shown in the Debrid settings screen.
func KnownProviders() []string {
	return []string{ProviderRealDebrid, ProviderAllDebrid, ProviderPremiumize, ProviderTorBox}
}

// DisplayName returns the human-readable name for a provider id.
func DisplayName(id string) string {
	switch id {
	case ProviderRealDebrid:
		return "Real-Debrid"
	case ProviderAllDebrid:
		return "AllDebrid"
	case ProviderPremiumize:
		return "Premiumize"
	case ProviderTorBox:
		return "TorBox"
	default:
		return id
	}
}

// New constructs the Provider for a known provider id and API token.
func New(id, token string) (Provider, error) {
	switch id {
	case ProviderRealDebrid:
		return NewRealDebrid(token), nil
	case ProviderAllDebrid:
		return NewAllDebrid(token), nil
	case ProviderPremiumize:
		return NewPremiumize(token), nil
	case ProviderTorBox:
		return NewTorBox(token), nil
	default:
		return nil, fmt.Errorf("unknown debrid provider %q", id)
	}
}

// Manager tries a set of configured providers in order until one resolves
// the requested magnet from its cache.
type Manager struct {
	providers []Provider
}

func NewManager(providers ...Provider) *Manager {
	return &Manager{providers: providers}
}

func (m *Manager) Empty() bool { return len(m.providers) == 0 }

// Resolve tries each configured provider in order and returns the first
// successful resolution along with which provider served it. If every
// provider fails (or none are configured), it returns a combined error.
func (m *Manager) Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (file ResolvedFile, providerName string, err error) {
	if len(m.providers) == 0 {
		return ResolvedFile{}, "", errors.New("no debrid provider configured")
	}

	var errs []error
	for _, p := range m.providers {
		f, err := p.Resolve(ctx, magnetOrHash, fileIdx)
		if err == nil {
			return f, p.Name(), nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
	}
	return ResolvedFile{}, "", errors.Join(errs...)
}
