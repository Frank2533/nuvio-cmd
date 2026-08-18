package debrid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllDebridResolveReadyImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want Bearer test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v4/magnet/upload":
			w.Write([]byte(`{"data":{"magnets":[{"id":42,"hash":"abc","ready":true}]}}`))

		case "/v4/magnet/files":
			if got := r.URL.Query().Get("id[]"); got != "42" {
				t.Errorf("magnet/files id[] = %q, want 42", got)
			}
			w.Write([]byte(`{
				"data": {"magnets": [{"files": [
					{"n": "sample.mp4", "s": 1000, "l": "https://alldebrid.com/dl/sample"},
					{"n": "Season 1", "e": [
						{"n": "movie.mkv", "s": 2000000000, "l": "https://alldebrid.com/dl/movie"}
					]}
				]}]}
			}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ad := &AllDebrid{c: newHTTPClient(srv.URL, "test-key")}
	got, err := ad.Resolve(context.Background(), "hash", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// No fileIdx given and two candidates -> largest file wins.
	if got.URL != "https://alldebrid.com/dl/movie" {
		t.Errorf("URL = %q, want the larger nested file's link", got.URL)
	}
	if got.Name != "movie.mkv" {
		t.Errorf("Name = %q, want movie.mkv", got.Name)
	}
}

func TestAllDebridResolveNotCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v4/magnet/upload":
			w.Write([]byte(`{"data":{"magnets":[{"id":7,"hash":"abc","ready":false}]}}`))
		case "/v4.1/magnet/status":
			w.Write([]byte(`{"data":{"magnets":{"statusCode":6,"status":"Error"}}}`))
		}
	}))
	defer srv.Close()

	ad := &AllDebrid{c: newHTTPClient(srv.URL, "test-key")}
	_, err := ad.Resolve(context.Background(), "hash", nil)
	if err == nil {
		t.Fatal("Resolve() with an errored magnet status should return an error")
	}
}

func TestFlattenAllDebridFiles(t *testing.T) {
	var nodes []adFileNode
	if err := json.Unmarshal([]byte(`[
		{"n": "a.mp4", "s": 1, "l": "link-a"},
		{"n": "folder", "e": [
			{"n": "b.mp4", "s": 2, "l": "link-b"},
			{"n": "subfolder", "e": [{"n": "c.mp4", "s": 3, "l": "link-c"}]}
		]}
	]`), &nodes); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	leaves := flattenAllDebridFiles(nodes)
	if len(leaves) != 3 {
		t.Fatalf("flattenAllDebridFiles: got %d leaves, want 3", len(leaves))
	}
}
