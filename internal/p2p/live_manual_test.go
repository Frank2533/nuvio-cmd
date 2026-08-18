//go:build manual

package p2p

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"nuvio-cmd/internal/player"
)

// Sintel is a Creative Commons short film widely used as a legal test fixture
// for BitTorrent/WebTorrent clients (see webtorrent.io/torrents).
const sintelMagnet = `magnet:?xt=urn:btih:08ada5a7a6183aae1e09d831df6748d566095a10&dn=Sintel&tr=udp://explodie.org:6969&tr=udp://tracker.coppersurfer.tk:6969&tr=udp://tracker.opentrackr.org:1337&tr=wss://tracker.btorrent.xyz&tr=wss://tracker.openwebtorrent.com`

func TestLiveP2PStreamSintel(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	stream, err := engine.Open(ctx, sintelMagnet, nil)
	if err != nil {
		t.Fatalf("Open: %v (this needs real DHT/tracker network access and active peers; "+
			"a sandboxed/firewalled environment may legitimately fail here even though the code is correct)", err)
	}
	defer stream.Close()

	t.Logf("opened stream: name=%q url=%s", stream.Name, stream.URL)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, stream.URL, nil)
	req.Header.Set("Range", "bytes=0-65535")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET stream URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 or 206", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	t.Logf("read %d bytes of real video data over the local HTTP bridge", len(data))
	if len(data) == 0 {
		t.Fatal("read 0 bytes — the local HTTP server isn't actually serving torrent data")
	}
}

// TestLiveP2PPlaybackViaMpv exercises the full pipeline this milestone
// exists for: real torrent -> local HTTP bridge -> real mpv, controlled the
// same way the TUI drives it.
func TestLiveP2PPlaybackViaMpv(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	stream, err := engine.Open(ctx, sintelMagnet, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()
	t.Logf("p2p stream ready: %s", stream.URL)

	p, err := player.Start(stream.URL, stream.Name)
	if err != nil {
		t.Fatalf("player.Start: %v", err)
	}
	defer p.Stop()

	var status player.Status
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		status = p.Status()
		if status.Position > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Logf("mpv status after startup: %+v", status)
	if status.Position <= 0 {
		t.Fatal("mpv time-pos never advanced past 0 — playback never actually started")
	}
}
