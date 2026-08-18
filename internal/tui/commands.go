package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"nuvio-cmd/internal/addon"
	"nuvio-cmd/internal/config"
	"nuvio-cmd/internal/debrid"
	"nuvio-cmd/internal/p2p"
	"nuvio-cmd/internal/player"
)

type addonsLoadedMsg struct{ states []addonState }

type catalogLoadedMsg struct {
	metas []addon.MetaPreview
	err   error
}

type metaLoadedMsg struct {
	meta *addon.MetaDetail
	err  error
}

type streamsLoadedMsg struct {
	streams []addon.Stream
	err     error
}

type addonAddedMsg struct {
	state addonState
}

type debridLoadedMsg struct{ entries []config.DebridEntry }

type debridSavedMsg struct {
	entries []config.DebridEntry
	err     error
}

type debridResolvedMsg struct {
	file  debrid.ResolvedFile
	title string
	err   error
}

type p2pConsentLoadedMsg struct{ given bool }

type p2pStreamOpenedMsg struct {
	engine *p2p.Engine
	stream *p2p.Stream
	err    error
}

type playerStartedMsg struct {
	p   *player.Player
	err error
}

type playerStatusMsg struct{ status player.Status }

type playerExitedMsg struct{}

type errMsg struct{ err error }

// loadAddonsCmd reads the configured addon list and fetches every enabled
// addon's manifest concurrently isn't necessary at this scale — sequential
// is simple and fast enough for a handful of addons.
func loadAddonsCmd(client *addon.Client) tea.Cmd {
	return func() tea.Msg {
		entries, err := config.LoadAddons()
		if err != nil {
			return errMsg{err}
		}

		states := make([]addonState, len(entries))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		for i, e := range entries {
			states[i] = addonState{entry: e}
			if !e.Enabled {
				continue
			}
			m, err := client.FetchManifest(ctx, e.ManifestURL)
			states[i].manifest = m
			states[i].err = err
		}
		return addonsLoadedMsg{states}
	}
}

func addAddonCmd(client *addon.Client, existing []config.AddonEntry, manifestURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		m, err := client.FetchManifest(ctx, manifestURL)
		entry := config.AddonEntry{ManifestURL: manifestURL, Enabled: true}
		if err == nil {
			updated := append(append([]config.AddonEntry{}, existing...), entry)
			err = config.SaveAddons(updated)
		}
		return addonAddedMsg{addonState{entry: entry, manifest: m, err: err}}
	}
}

func fetchCatalogCmd(client *addon.Client, m *addon.Manifest, typ, id string, extra map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		metas, err := client.FetchCatalog(ctx, m, typ, id, extra)
		return catalogLoadedMsg{metas, err}
	}
}

func fetchMetaCmd(client *addon.Client, m *addon.Manifest, typ, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		meta, err := client.FetchMeta(ctx, m, typ, id)
		return metaLoadedMsg{meta, err}
	}
}

// fetchStreamsCmd queries every stream-capable addon for id, not just the
// addon that served the catalog/meta: in the Stremio ecosystem, catalog and
// stream resources typically come from different addons (Cinemeta, for
// example, only serves catalog/meta and has no stream resource at all).
func fetchStreamsCmd(client *addon.Client, manifests []*addon.Manifest, typ, id string) tea.Cmd {
	return func() tea.Msg {
		if len(manifests) == 0 {
			return streamsLoadedMsg{nil, fmt.Errorf("no installed addon provides streams for %q — add a stream addon under Addons", typ)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var all []addon.Stream
		var lastErr error
		for _, m := range manifests {
			streams, err := client.FetchStreams(ctx, m, typ, id)
			if err != nil {
				lastErr = err
				continue
			}
			all = append(all, streams...)
		}
		if len(all) == 0 && lastErr != nil {
			return streamsLoadedMsg{nil, lastErr}
		}
		return streamsLoadedMsg{all, nil}
	}
}

func loadDebridCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := config.LoadDebrid()
		if err != nil {
			return errMsg{err}
		}
		return debridLoadedMsg{entries}
	}
}

func saveDebridCmd(entries []config.DebridEntry) tea.Cmd {
	return func() tea.Msg {
		err := config.SaveDebrid(entries)
		return debridSavedMsg{entries, err}
	}
}

// resolveDebridCmd resolves an infoHash-only stream through the configured
// debrid providers, in priority order, into a directly playable URL.
func resolveDebridCmd(manager *debrid.Manager, stream addon.Stream) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		file, _, err := manager.Resolve(ctx, stream.InfoHash, stream.FileIdx)
		return debridResolvedMsg{file: file, title: stream.DisplayTitle(), err: err}
	}
}

func loadP2PConsentCmd() tea.Cmd {
	return func() tea.Msg {
		given, err := config.LoadP2PConsent()
		if err != nil {
			return errMsg{err}
		}
		return p2pConsentLoadedMsg{given}
	}
}

func saveP2PConsentCmd(given bool) tea.Cmd {
	return func() tea.Msg {
		_ = config.SaveP2PConsent(given) // best-effort: worst case, we ask again next session
		return nil
	}
}

// openP2PStreamCmd lazily creates the P2P engine on first use (starting a
// torrent.Client opens listening sockets and begins DHT bootstrap, which
// shouldn't happen for users who never touch this feature), then opens
// stream for playback. The engine is always returned, even on failure, so
// the caller can keep reusing it rather than leaking a new Client per
// attempt.
func openP2PStreamCmd(engine *p2p.Engine, stream addon.Stream) tea.Cmd {
	return func() tea.Msg {
		var err error
		if engine == nil {
			engine, err = p2p.NewEngine()
			if err != nil {
				return p2pStreamOpenedMsg{err: err}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		s, err := engine.Open(ctx, stream.InfoHash, stream.FileIdx)
		return p2pStreamOpenedMsg{engine: engine, stream: s, err: err}
	}
}

func startPlayerCmd(streamURL, title string) tea.Cmd {
	return func() tea.Msg {
		p, err := player.Start(streamURL, title)
		return playerStartedMsg{p, err}
	}
}

// waitPlayerCmd blocks until mpv exits (user closed it, or playback ended).
func waitPlayerCmd(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		_ = p.Wait()
		return playerExitedMsg{}
	}
}

// pollPlayerStatusCmd polls once, one second from now. The caller
// reschedules it after each result to build a self-perpetuating tick loop
// for as long as the player screen is active.
func pollPlayerStatusCmd(p *player.Player) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return playerStatusMsg{p.Status()}
	})
}

func stopPlayerCmd(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		p.Stop()
		return playerExitedMsg{}
	}
}
