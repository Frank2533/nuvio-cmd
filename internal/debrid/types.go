// Package debrid resolves a torrent (magnet/infoHash) into a direct,
// mpv-playable HTTP URL via a "debrid" caching service — Real-Debrid,
// AllDebrid, Premiumize, or TorBox. These are the services that make an
// infoHash-only stream from an addon (e.g. Torrentio) playable without
// Nuvio CMD needing its own BitTorrent engine (that's the separate, later
// P2P milestone).
//
// Every provider here uses a documented, official public REST API, and each
// client's request/response shapes were checked against that provider's own
// API docs rather than guessed — except TorBox, whose docs are a
// JS-rendered SPA that couldn't be fully machine-verified; its field names
// are flagged inline where they're best-effort rather than confirmed.
package debrid

import (
	"context"
	"errors"
	"strings"
)

// ErrNotCached means the provider doesn't already have this torrent ready to
// serve instantly. Nuvio CMD only uses debrid providers for their instant
// cache — it does not wait around for a provider to download a torrent from
// scratch, since that can take arbitrarily long and isn't what "debrid
// streaming" means to a user picking a stream to watch right now.
var ErrNotCached = errors.New("not cached by this debrid provider")

// ResolvedFile is one playable file a provider resolved a magnet into.
type ResolvedFile struct {
	Name string
	URL  string
	Size int64
}

// Provider resolves a magnet/infoHash into a directly playable file.
type Provider interface {
	// Name is the human-readable provider name, e.g. "Real-Debrid".
	Name() string

	// Resolve looks up magnetOrHash in the provider's cache and returns a
	// direct URL for one file within it. fileIdx, if non-nil, selects which
	// file (0-based, matching the addon stream's FileIdx); if nil, or if
	// the provider can't map indices reliably, the single file (or the
	// largest, if there's more than one candidate) is returned instead.
	// Returns ErrNotCached if the provider would have to download the
	// torrent first rather than serving it instantly.
	Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (ResolvedFile, error)
}

// MagnetFromHash builds a minimal magnet URI from a bare BitTorrent
// infoHash, which is the form addon Stream.InfoHash values come in.
func MagnetFromHash(hash string) string {
	return "magnet:?xt=urn:btih:" + strings.ToUpper(hash)
}
