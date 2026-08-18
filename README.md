# Nuvio CMD

An unofficial, cross-platform (Windows/macOS/Linux) terminal client for browsing and
playing media through Stremio-protocol addons — inspired by, but not affiliated with,
[NuvioDesktop](https://github.com/NuvioMedia/NuvioDesktop).

Status: **M3** — browse Stremio-protocol addon catalogs, view details, and play
streams through mpv: directly, resolved through a debrid provider's instant
cache (Real-Debrid, AllDebrid, Premiumize, TorBox), or as a last resort
streamed live over BitTorrent (with an explicit consent screen first). Addon
and debrid management (add/list/configure) are in. Tracking sync, downloads,
and library are later milestones. See the plan for the full milestone list
and architecture rationale.

## Run from source

```bash
go run ./cmd/nuvio
```

Requires Go 1.24+, and `mpv` installed on your system for playback (not bundled).
On first run it seeds the addon list with the official Cinemeta catalog addon;
add stream-capable addons from the Addons screen (`a`) to get playable streams
— Cinemeta itself only serves catalog/meta, not streams. For a torrent-only
stream (infoHash, no direct URL), Nuvio CMD tries any configured debrid
provider's instant cache first, then falls back to streaming it directly over
BitTorrent — the first time that happens it shows a one-time consent screen
(your IP is visible to other peers in the swarm; you're responsible for the
legality of what you stream) before proceeding.

## Architecture

- `cmd/nuvio` — entrypoint
- `internal/tui` — Bubble Tea screens (menu, addons, debrid, browse, details, streams, P2P consent, player)
- `internal/addon` — Stremio-protocol addon client (manifest/catalog/meta/stream)
- `internal/config` — on-disk addon/debrid/P2P-consent config persistence
- `internal/debrid` — Real-Debrid/AllDebrid/Premiumize/TorBox clients, resolving a magnet/infoHash into a direct URL via each provider's instant cache
- `internal/p2p` — BitTorrent streaming (anacrolix/torrent), re-serving one selected file over a local Range-aware HTTP server mpv plays like any other URL
- `internal/player` — mpv process + JSON IPC control (Unix socket / Windows named pipe)
- `internal/tmdb`, `internal/tracking` — metadata and Trakt/Simkl/MDBList sync (not yet implemented)
- `internal/library`, `internal/downloads` — local state (not yet implemented)

## Testing

`go test ./...` runs the committed suite (protocol-shape tests for `addon`
using recorded fixtures, per-provider `debrid` tests against fake HTTP
servers, a fake-IPC-server test for `player`, config round-trip tests) — no
network, mpv, or real debrid accounts required, safe for CI. Real-Debrid,
AllDebrid, and Premiumize request/response shapes were checked against each
provider's official API docs; TorBox's docs are a JS-rendered SPA that
couldn't be fully verified, so its client parses defensively and is flagged
inline as best-effort pending a real-account check.

Files tagged `manual` (`go test -tags manual -run TestLive... ./...`) are
live, network/mpv-dependent smoke tests against the real Cinemeta addon, a
real mpv process, and (for `p2p`) a real BitTorrent swarm via a legal
Creative Commons test torrent (Sintel, from webtorrent.io/torrents). They're
excluded from the default build and CI, and are meant to be run by hand when
verifying a milestone end-to-end. The `p2p` live test is also how a real bug
was caught during development: an earlier version shared one `torrent.Reader`
across all HTTP requests behind a mutex, which serialized real players'
overlapping range requests (ffmpeg probing for the MP4 `moov` atom while
still reading the front of the file) badly enough to make playback fail
against a real torrent even though every unit test and a synthetic curl
check passed. The fix (an independent reader per request) is covered by a
committed regression test (`TestStreamServeConcurrentRequestsDontInterfere`)
that doesn't need network access.

## Status of parity with NuvioDesktop

This is a from-scratch reimplementation against public protocols (TMDB, the Stremio
addon spec, debrid provider APIs, Trakt/Simkl/MDBList), not a port of NuvioDesktop's
Kotlin source, so it isn't GPL-3.0 encumbered. NuvioDesktop's proprietary encrypted
plugin/scraper format is out of scope until the rest of the client is solid — see the
plan's M7 for details.
