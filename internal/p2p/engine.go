// Package p2p streams BitTorrent content directly, for addon streams that
// only give an infoHash (no direct URL) and that no configured debrid
// provider could resolve from its cache. It wraps anacrolix/torrent,
// selecting one file for sequential ("responsive") download and re-serving
// it over a local, Range-aware HTTP server that mpv plays like any other
// URL — mpv never talks to the BitTorrent swarm directly.
package p2p

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
)

// Engine owns the anacrolix/torrent Client for the process. It's meant to
// be created once (lazily, on first P2P use — starting it opens listening
// sockets and begins DHT bootstrap, which shouldn't happen for users who
// never touch this feature) and reused across streams; only one Stream is
// expected to be active at a time, mirroring the rest of the app's
// single-now-playing design.
type Engine struct {
	client  *torrent.Client
	dataDir string
}

func NewEngine() (*Engine, error) {
	dataDir := filepath.Join(os.TempDir(), "nuvio-cmd-p2p")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("p2p: create data dir: %w", err)
	}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dataDir

	cl, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("p2p: start torrent client: %w", err)
	}
	return &Engine{client: cl, dataDir: dataDir}, nil
}

// Close shuts down the torrent client and removes any data it downloaded.
func (e *Engine) Close() {
	e.client.Close()
	_ = os.RemoveAll(e.dataDir)
}

func magnetFrom(magnetOrHash string) string {
	if len(magnetOrHash) == 40 || len(magnetOrHash) == 32 {
		return "magnet:?xt=urn:btih:" + strings.ToUpper(magnetOrHash)
	}
	return magnetOrHash
}

// Open adds magnetOrHash, waits for its metadata, selects fileIdx (or the
// largest file if nil/out of range), and starts serving that file over a
// local HTTP server. The returned Stream's URL is ready to hand to mpv.
func (e *Engine) Open(ctx context.Context, magnetOrHash string, fileIdx *int) (*Stream, error) {
	t, err := e.client.AddMagnet(magnetFrom(magnetOrHash))
	if err != nil {
		return nil, fmt.Errorf("p2p: add magnet: %w", err)
	}

	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		t.Drop()
		return nil, ctx.Err()
	}

	files := t.Files()
	if len(files) == 0 {
		t.Drop()
		return nil, fmt.Errorf("p2p: torrent has no files")
	}
	chosen := files[0]
	if fileIdx != nil && *fileIdx >= 0 && *fileIdx < len(files) {
		chosen = files[*fileIdx]
	} else {
		for _, f := range files[1:] {
			if f.Length() > chosen.Length() {
				chosen = f
			}
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Drop()
		return nil, fmt.Errorf("p2p: listen: %w", err)
	}

	s := &Stream{
		Name:        filepath.Base(chosen.Path()),
		torrent:     t,
		file:        chosen,
		ln:          ln,
		engineData:  e.dataDir,
		torrentName: t.Name(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", s.serve)
	s.srv = &http.Server{Handler: mux}
	go s.srv.Serve(ln)

	s.URL = fmt.Sprintf("http://%s/stream", ln.Addr().String())
	return s, nil
}

// Stream is one torrent file being served locally for playback.
type Stream struct {
	Name string
	URL  string

	torrent     *torrent.Torrent
	file        fileSource
	ln          net.Listener
	srv         *http.Server
	engineData  string
	torrentName string
}

// fileSource is the slice of *torrent.File that serve needs. Extracted as
// an interface (which *torrent.File satisfies structurally, no wrapping
// needed) so tests can exercise the HTTP/Range-serving logic with a fake,
// without a real BitTorrent swarm.
type fileSource interface {
	NewReader() torrent.Reader
	Length() int64
}

// serve gives each HTTP request its own torrent.Reader rather than sharing
// one: torrent.Reader is documented as not safe for concurrent use, and
// real players routinely issue multiple overlapping range requests (e.g.
// ffmpeg's MP4 demuxer probing for the moov atom while still reading from
// the front of the file). Multiple readers over the same File are exactly
// what anacrolix/torrent expects for this — their priorities merge in the
// underlying torrent rather than fighting each other.
func (s *Stream) serve(w http.ResponseWriter, r *http.Request) {
	reader := s.file.NewReader()
	defer reader.Close()
	reader.SetResponsive()
	if l := s.file.Length(); l > 0 {
		reader.SetReadahead(l / 20) // ~5% of the file: enough buffer ahead of playback without over-prioritizing the whole download
	}
	http.ServeContent(w, r, s.Name, time.Time{}, reader)
}

// Close stops serving, drops the torrent (halting further downloads), and
// best-effort removes the data it had already written to disk.
func (s *Stream) Close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
	if s.torrent != nil {
		s.torrent.Drop()
	}
	if s.engineData != "" && s.torrentName != "" {
		_ = os.RemoveAll(filepath.Join(s.engineData, s.torrentName))
	}
}
