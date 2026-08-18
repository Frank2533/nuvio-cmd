package addon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchManifest(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer srv.Close()

	c := NewClient()
	m, err := c.FetchManifest(context.Background(), srv.URL+"/manifest.json")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}

	if m.ID != "com.linvo.cinemeta" {
		t.Errorf("ID = %q, want com.linvo.cinemeta", m.ID)
	}
	if m.TransportURL != srv.URL {
		t.Errorf("TransportURL = %q, want %q", m.TransportURL, srv.URL)
	}
	if !m.HasResource("catalog") {
		t.Error("HasResource(catalog) = false, want true (string-form resources)")
	}
	if m.HasResource("stream") {
		t.Error("HasResource(stream) = true, want false")
	}

	if len(m.Catalogs) != 2 {
		t.Fatalf("len(Catalogs) = %d, want 2", len(m.Catalogs))
	}

	structured := m.Catalogs[0]
	if !structured.SupportsSearch() {
		t.Error("structured catalog: SupportsSearch() = false, want true")
	}
	if got := structured.NormalizedExtra(); len(got) != 3 {
		t.Errorf("structured catalog NormalizedExtra() len = %d, want 3", len(got))
	}

	legacy := m.Catalogs[1]
	extra := legacy.NormalizedExtra()
	if len(extra) != 2 {
		t.Fatalf("legacy catalog NormalizedExtra() len = %d, want 2", len(extra))
	}
	if !legacy.SupportsSearch() {
		t.Error("legacy catalog: SupportsSearch() = false, want true (synthesized from extraSupported)")
	}
	var searchRequired bool
	for _, e := range extra {
		if e.Name == "search" {
			searchRequired = e.IsRequired
		}
	}
	if !searchRequired {
		t.Error("legacy catalog: search extra should be required per extraRequired")
	}
}

func TestFetchCatalogAndMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/catalog/movie/top/search=inception&skip=10.json":
			http.ServeFile(w, r, "testdata/catalog.json")
		case "/meta/series/tt0944947.json":
			http.ServeFile(w, r, "testdata/meta.json")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient()
	manifest := &Manifest{TransportURL: srv.URL}

	metas, err := c.FetchCatalog(context.Background(), manifest, "movie", "top", map[string]string{
		"search": "inception",
		"skip":   "10",
	})
	if err != nil {
		t.Fatalf("FetchCatalog: %v", err)
	}
	if len(metas) != 1 || metas[0].Name != "Inception" {
		t.Fatalf("FetchCatalog metas = %+v, want [Inception]", metas)
	}

	meta, err := c.FetchMeta(context.Background(), manifest, "series", "tt0944947")
	if err != nil {
		t.Fatalf("FetchMeta: %v", err)
	}
	if meta.Name != "Game of Thrones" {
		t.Errorf("meta.Name = %q, want Game of Thrones", meta.Name)
	}
	if len(meta.Videos) != 1 || meta.Videos[0].ID != "tt0944947:1:1" {
		t.Fatalf("meta.Videos = %+v, want one video with composite id", meta.Videos)
	}
}

func TestEncodeExtra(t *testing.T) {
	got := EncodeExtra(map[string]string{"skip": "10", "search": "the matrix"})
	want := "search=the+matrix&skip=10"
	if got != want {
		t.Errorf("EncodeExtra = %q, want %q", got, want)
	}
	if got := EncodeExtra(nil); got != "" {
		t.Errorf("EncodeExtra(nil) = %q, want empty", got)
	}
}

func TestStreamPlayable(t *testing.T) {
	direct := Stream{URL: "https://example.com/movie.mp4", Title: "1080p"}
	if !direct.Playable() {
		t.Error("direct URL stream should be Playable")
	}
	torrent := Stream{InfoHash: "abcd1234", Title: "1080p WEB-DL"}
	if torrent.Playable() {
		t.Error("infoHash-only stream should not be Playable yet (needs M3 P2P engine)")
	}
	if got := torrent.DisplayTitle(); got != "1080p WEB-DL" {
		t.Errorf("DisplayTitle() = %q, want title", got)
	}
}
