package p2p

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/anacrolix/torrent"
)

// fakeReader implements the anacrolix torrent.Reader interface over an
// in-memory buffer, so Stream.serve's HTTP/Range handling can be tested
// without a real BitTorrent swarm.
type fakeReader struct {
	*bytes.Reader
}

func (f fakeReader) Close() error                                         { return nil }
func (f fakeReader) SetContext(context.Context)                           {}
func (f fakeReader) ReadContext(_ context.Context, p []byte) (int, error) { return f.Read(p) }
func (f fakeReader) SetReadahead(int64)                                   {}
func (f fakeReader) SetReadaheadFunc(torrent.ReadaheadFunc)               {}
func (f fakeReader) SetResponsive()                                       {}

var _ torrent.Reader = fakeReader{}

// fakeFileSource stands in for *torrent.File: each NewReader() call returns
// an independent reader starting at offset 0 over the same content, which
// is exactly the "multiple concurrent readers over one file" behavior
// serve() depends on (see the comment on fileSource).
type fakeFileSource struct {
	content       string
	readersIssued atomic.Int64
}

func (f *fakeFileSource) NewReader() torrent.Reader {
	f.readersIssued.Add(1)
	return fakeReader{bytes.NewReader([]byte(f.content))}
}

func (f *fakeFileSource) Length() int64 { return int64(len(f.content)) }

func newTestStream(t *testing.T, content string) (*Stream, *fakeFileSource, *httptest.Server) {
	t.Helper()
	src := &fakeFileSource{content: content}
	s := &Stream{
		Name: "movie.mp4",
		file: src,
	}
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(srv.Close)
	return s, src, srv
}

func TestStreamServeFullContent(t *testing.T) {
	content := "hello from the swarm"
	_, _, srv := newTestStream(t, content)

	resp, err := http.Get(srv.URL + "/stream")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != content {
		t.Errorf("body = %q, want %q", body, content)
	}
	if got := resp.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes (needed for player seeking)", got)
	}
}

func TestStreamServeRangeRequest(t *testing.T) {
	content := "0123456789abcdefghij"
	_, _, srv := newTestStream(t, content)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/stream", nil)
	req.Header.Set("Range", "bytes=5-9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET with Range: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206 Partial Content", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "56789" {
		t.Errorf("ranged body = %q, want %q", body, "56789")
	}
}

// TestStreamServeConcurrentRequestsDontInterfere is a regression test: an
// earlier version shared one torrent.Reader across all requests behind a
// mutex, which serialized real players' overlapping range requests (e.g.
// ffmpeg probing for the MP4 moov atom while still reading the front of the
// file) badly enough to make playback fail against a real torrent. Each
// request must get its own independent reader.
func TestStreamServeConcurrentRequestsDontInterfere(t *testing.T) {
	content := "0123456789abcdefghijklmnopqrstuvwxyz"
	_, src, srv := newTestStream(t, content)

	fetch := func(rng string) (string, error) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/stream", nil)
		req.Header.Set("Range", rng)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}

	type result struct {
		body string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		body, err := fetch("bytes=0-4") // "01234", near the start
		results <- result{body, err}
	}()
	go func() {
		body, err := fetch("bytes=30-35") // "uvwxyz", near the end
		results <- result{body, err}
	}()

	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("concurrent fetch: %v", r.err)
		}
		got[r.body] = true
	}
	if !got["01234"] || !got["uvwxyz"] {
		t.Fatalf("concurrent requests interfered with each other: got %v, want both 01234 and uvwxyz", got)
	}
	if n := src.readersIssued.Load(); n != 2 {
		t.Errorf("readersIssued = %d, want 2 (one independent reader per request)", n)
	}
}

func TestMagnetFrom(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef01234567"[:40]
	got := magnetFrom(hash)
	want := "magnet:?xt=urn:btih:" + "0123456789ABCDEF0123456789ABCDEF01234567"
	if got != want {
		t.Errorf("magnetFrom(bare hash) = %q, want %q", got, want)
	}

	already := "magnet:?xt=urn:btih:abc&dn=Example"
	if got := magnetFrom(already); got != already {
		t.Errorf("magnetFrom(magnet URI) = %q, want unchanged %q", got, already)
	}
}
