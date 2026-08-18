package debrid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealDebridResolve(t *testing.T) {
	var selectFilesCalled string
	status := "waiting_files_selection"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/torrents/addMagnet":
			json.NewEncoder(w).Encode(map[string]string{"id": "abc123"})

		case r.Method == http.MethodPost && r.URL.Path == "/torrents/selectFiles/abc123":
			_ = r.ParseForm()
			selectFilesCalled = r.Form.Get("files")
			status = "downloaded" // simulate becoming ready right after selection
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodGet && r.URL.Path == "/torrents/info/abc123":
			info := rdTorrentInfo{
				ID:     "abc123",
				Status: status,
				Files: []rdFile{
					{ID: 1, Path: "/sample.mp4", Bytes: 1000},
					{ID: 2, Path: "/movie.mkv", Bytes: 2_000_000_000},
				},
			}
			if status == "downloaded" {
				info.Links = []string{"https://real-debrid.com/d/hosted-link"}
			}
			json.NewEncoder(w).Encode(info)

		case r.Method == http.MethodPost && r.URL.Path == "/unrestrict/link":
			_ = r.ParseForm()
			if got := r.Form.Get("link"); got != "https://real-debrid.com/d/hosted-link" {
				t.Errorf("unrestrict link = %q, want the hosted link", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"download": "https://direct.real-debrid.com/movie.mkv",
				"filename": "movie.mkv",
				"filesize": 2_000_000_000,
			})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	rd := &RealDebrid{c: newHTTPClient(srv.URL, "test-token")}
	fileIdx := 1
	got, err := rd.Resolve(context.Background(), "somehash1234567890somehash1234567890ab", &fileIdx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.URL != "https://direct.real-debrid.com/movie.mkv" {
		t.Errorf("URL = %q, want direct link", got.URL)
	}
	if selectFilesCalled != "2" {
		t.Errorf("selectFiles called with files=%q, want %q (file id for fileIdx=1)", selectFilesCalled, "2")
	}
}

func TestRealDebridResolveErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/torrents/addMagnet":
			json.NewEncoder(w).Encode(map[string]string{"id": "xyz"})
		case r.URL.Path == "/torrents/selectFiles/xyz":
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/torrents/info/xyz":
			// A terminal error status should surface immediately, without
			// Resolve waiting out its poll deadline.
			json.NewEncoder(w).Encode(rdTorrentInfo{ID: "xyz", Status: "dead"})
		}
	}))
	defer srv.Close()

	rd := &RealDebrid{c: newHTTPClient(srv.URL, "test-token")}
	_, err := rd.Resolve(context.Background(), "hash", nil)
	if err == nil {
		t.Fatal("Resolve() with a dead torrent status should return an error")
	}
}
