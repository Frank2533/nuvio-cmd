//go:build manual

package addon

import (
	"context"
	"testing"
)

func TestLiveCinemeta(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	m, err := c.FetchManifest(ctx, "https://v3-cinemeta.strem.io/manifest.json")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	t.Logf("manifest: %s v%s, %d catalogs", m.Name, m.Version, len(m.Catalogs))
	if len(m.Catalogs) == 0 {
		t.Fatal("expected catalogs")
	}

	cat := m.Catalogs[0]
	metas, err := c.FetchCatalog(ctx, m, cat.Type, cat.ID, nil)
	if err != nil {
		t.Fatalf("FetchCatalog: %v", err)
	}
	t.Logf("catalog %q: %d items, first = %q", cat.Name, len(metas), metas[0].Name)
	if len(metas) == 0 {
		t.Fatal("expected catalog items")
	}

	meta, err := c.FetchMeta(ctx, m, metas[0].Type, metas[0].ID)
	if err != nil {
		t.Fatalf("FetchMeta: %v", err)
	}
	t.Logf("meta: %s — %.80s...", meta.Name, meta.Description)

	searchResults, err := c.FetchCatalog(ctx, m, "movie", "top", map[string]string{"search": "inception"})
	if err != nil {
		t.Fatalf("FetchCatalog(search): %v", err)
	}
	t.Logf("search 'inception': %d results, first = %q", len(searchResults), searchResults[0].Name)
	if len(searchResults) == 0 || searchResults[0].Name != "Inception" {
		t.Fatalf("expected Inception as first search result, got %+v", searchResults)
	}
}
