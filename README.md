# Nuvio CMD

An unofficial, cross-platform (Windows/macOS/Linux) terminal client for browsing and
playing media through Stremio-protocol addons — inspired by, but not affiliated with,
[NuvioDesktop](https://github.com/NuvioMedia/NuvioDesktop).

Status: **M0 scaffold**. See the plan for milestones and architecture.

## Run from source

```bash
go run ./cmd/nuvio
```

Requires Go 1.24+, and `mpv` installed on your system for playback (not bundled).

## Architecture

- `cmd/nuvio` — entrypoint
- `internal/tui` — Bubble Tea screens
- `internal/addon` — Stremio-protocol addon client
- `internal/tmdb`, `internal/tracking` — metadata and Trakt/Simkl/MDBList sync
- `internal/debrid` — Real-Debrid/AllDebrid/Premiumize/TorBox clients
- `internal/p2p` — torrent streaming
- `internal/player` — mpv process + JSON IPC control
- `internal/library`, `internal/downloads` — local state

## Status of parity with NuvioDesktop

This is a from-scratch reimplementation against public protocols (TMDB, the Stremio
addon spec, debrid provider APIs, Trakt/Simkl/MDBList), not a port of NuvioDesktop's
Kotlin source, so it isn't GPL-3.0 encumbered. NuvioDesktop's proprietary encrypted
plugin/scraper format is out of scope until the rest of the client is solid — see the
plan's M7 for details.
