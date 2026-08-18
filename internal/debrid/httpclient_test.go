package debrid

import (
	"encoding/json"
	"testing"
)

func TestFirstNonEmptyString(t *testing.T) {
	raw := json.RawMessage(`{"torrent_id": 42, "name": "movie.mkv", "empty": ""}`)

	if got := firstNonEmptyString(raw, "missing_key", "torrent_id"); got != "42" {
		t.Errorf("firstNonEmptyString(numeric id) = %q, want %q", got, "42")
	}
	if got := firstNonEmptyString(raw, "name"); got != "movie.mkv" {
		t.Errorf("firstNonEmptyString(string) = %q, want movie.mkv", got)
	}
	if got := firstNonEmptyString(raw, "empty", "name"); got != "movie.mkv" {
		t.Errorf("firstNonEmptyString should skip an empty match and try the next key, got %q", got)
	}
	if got := firstNonEmptyString(raw, "nope"); got != "" {
		t.Errorf("firstNonEmptyString(no match) = %q, want empty", got)
	}
}
