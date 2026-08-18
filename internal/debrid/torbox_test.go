package debrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTorBoxResolveReadyImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/torrents/createtorrent":
			w.Write([]byte(`{"data":{"torrent_id":99}}`))
		case "/torrents/mylist":
			if got := r.URL.Query().Get("id"); got != "99" {
				t.Errorf("mylist id = %q, want 99", got)
			}
			w.Write([]byte(`{"data":[
				{"id": 99, "files": [
					{"id": 1, "short_name": "sample.mp4"},
					{"id": 2, "short_name": "movie.mkv"}
				]}
			]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// TorBox's client hardcodes the real base URL into the requestdl link
	// (since redirect=true makes that URL itself directly playable — see
	// the package doc), so this only exercises createtorrent/mylist against
	// the fake server via c, and checks the constructed download URL shape.
	tb := &TorBox{c: newHTTPClient(srv.URL, "test-token"), token: "test-token"}
	fileIdx := 1
	got, err := tb.Resolve(context.Background(), "hash", &fileIdx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Name != "movie.mkv" {
		t.Errorf("Name = %q, want movie.mkv", got.Name)
	}
	if !strings.Contains(got.URL, "torrent_id=99") || !strings.Contains(got.URL, "file_id=2") {
		t.Errorf("URL = %q, want it to reference torrent_id=99 and file_id=2", got.URL)
	}
}
