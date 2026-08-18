package debrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPremiumizeResolveCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want Bearer test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/cache/check":
			w.Write([]byte(`{"response":[true]}`))
		case "/transfer/directdl":
			w.Write([]byte(`{
				"status": "success",
				"content": [
					{"path": "sample.mp4", "size": 1000, "link": "https://premiumize.me/dl/sample"},
					{"path": "movie.mkv", "size": 3000000000, "link": "https://premiumize.me/dl/movie"}
				]
			}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	pm := &Premiumize{c: newHTTPClient(srv.URL, "test-key")}
	fileIdx := 1
	got, err := pm.Resolve(context.Background(), "hash", &fileIdx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.URL != "https://premiumize.me/dl/movie" {
		t.Errorf("URL = %q, want the file at fileIdx=1", got.URL)
	}
}

func TestPremiumizeResolveNotCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/cache/check" {
			w.Write([]byte(`{"response":[false]}`))
		}
	}))
	defer srv.Close()

	pm := &Premiumize{c: newHTTPClient(srv.URL, "test-key")}
	_, err := pm.Resolve(context.Background(), "hash", nil)
	if err != ErrNotCached {
		t.Fatalf("Resolve() = %v, want ErrNotCached", err)
	}
}
