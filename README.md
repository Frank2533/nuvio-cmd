# Nuvio CMD

An unofficial, cross-platform (Windows/macOS/Linux) terminal client for browsing and
playing media through Stremio-protocol addons — inspired by, but not affiliated with,
[NuvioDesktop](https://github.com/NuvioMedia/NuvioDesktop).

Status: **M2** — browse Stremio-protocol addon catalogs, view details, and play
streams through mpv, either directly or resolved through a debrid provider's
instant cache (Real-Debrid, AllDebrid, Premiumize, TorBox). Addon and debrid
management (add/list/configure) are in. P2P streaming, tracking sync,
downloads, and library are later milestones. See the plan for the full
milestone list and architecture rationale.

## Run from source

```bash
go run ./cmd/nuvio
```

Requires Go 1.24+, and `mpv` installed on your system for playback (not bundled).
On first run it seeds the addon list with the official Cinemeta catalog addon;
add stream-capable addons from the Addons screen (`a`) to get playable streams
— Cinemeta itself only serves catalog/meta, not streams. If a stream is
torrent-only (infoHash, no direct URL), add at least one debrid provider's API
token from the Debrid screen to resolve it into a direct URL; without one (and
until P2P streaming lands in a later milestone), such streams can't be played.

## Architecture

- `cmd/nuvio` — entrypoint
- `internal/tui` — Bubble Tea screens (menu, addons, debrid, browse, details, streams, player)
- `internal/addon` — Stremio-protocol addon client (manifest/catalog/meta/stream)
- `internal/config` — on-disk addon/debrid config persistence
- `internal/debrid` — Real-Debrid/AllDebrid/Premiumize/TorBox clients, resolving a magnet/infoHash into a direct URL via each provider's instant cache
- `internal/player` — mpv process + JSON IPC control (Unix socket / Windows named pipe)
- `internal/tmdb`, `internal/tracking` — metadata and Trakt/Simkl/MDBList sync (not yet implemented)
- `internal/p2p` — torrent streaming (not yet implemented)
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
live, network/mpv-dependent smoke tests against the real Cinemeta addon and a
real mpv process. They're excluded from the default build and CI, and are
meant to be run by hand when verifying a milestone end-to-end.

## Status of parity with NuvioDesktop

This is a from-scratch reimplementation against public protocols (TMDB, the Stremio
addon spec, debrid provider APIs, Trakt/Simkl/MDBList), not a port of NuvioDesktop's
Kotlin source, so it isn't GPL-3.0 encumbered. NuvioDesktop's proprietary encrypted
plugin/scraper format is out of scope until the rest of the client is solid — see the
plan's M7 for details.
