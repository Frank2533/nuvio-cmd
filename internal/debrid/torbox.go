package debrid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// TorBox implements Provider against api.torbox.app/v1/api.
//
// Unlike the other three providers, TorBox's own docs are a JS-rendered SPA
// that couldn't be fully machine-verified against raw JSON examples, so the
// exact field names inside "mylist" torrent/file objects are best-effort
// rather than confirmed. To stay correct even if the real names differ
// slightly from what's guessed here, file name/id lookups go through
// firstNonEmptyString/firstNonZeroInt probing several plausible key names
// instead of a single hardcoded json tag. If this turns out wrong against a
// real account, the fix is narrowly scoped to torboxFile's probed keys.
type TorBox struct {
	c     *httpClient
	token string
}

func NewTorBox(token string) *TorBox {
	return &TorBox{c: newHTTPClient("https://api.torbox.app/v1/api", token), token: token}
}

func (t *TorBox) Name() string { return "TorBox" }

type torboxCreateResponse struct {
	Data json.RawMessage `json:"data"`
}

type torboxListResponse struct {
	Data []json.RawMessage `json:"data"`
}

func (t *TorBox) Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (ResolvedFile, error) {
	magnet := magnetOrHash
	if len(magnet) == 40 || len(magnet) == 32 {
		magnet = MagnetFromHash(magnet)
	}

	var created torboxCreateResponse
	if err := t.c.post(ctx, "/torrents/createtorrent", url.Values{"magnet": {magnet}}, &created); err != nil {
		return ResolvedFile{}, fmt.Errorf("torbox createtorrent: %w", err)
	}
	id := firstNonEmptyString(created.Data, "torrent_id", "id")
	if id == "" {
		return ResolvedFile{}, fmt.Errorf("torbox: could not determine torrent id from createtorrent response")
	}

	torrent, err := t.pollCached(ctx, id)
	if err != nil {
		return ResolvedFile{}, err
	}

	var files []json.RawMessage
	if err := json.Unmarshal(torrent, &struct {
		Files *[]json.RawMessage `json:"files"`
	}{Files: &files}); err != nil || len(files) == 0 {
		return ResolvedFile{}, fmt.Errorf("torbox: torrent has no files")
	}

	type candidate struct {
		fileID string
		name   string
	}
	var candidates []candidate
	for _, f := range files {
		fid := firstNonEmptyString(f, "id", "file_id")
		name := firstNonEmptyString(f, "short_name", "name", "path")
		if fid != "" {
			candidates = append(candidates, candidate{fileID: fid, name: name})
		}
	}
	if len(candidates) == 0 {
		return ResolvedFile{}, fmt.Errorf("torbox: could not determine any file id from mylist response")
	}

	idx := 0
	if fileIdx != nil && *fileIdx >= 0 && *fileIdx < len(candidates) {
		idx = *fileIdx
	}
	chosen := candidates[idx]

	dlURL := fmt.Sprintf("%s/torrents/requestdl?token=%s&torrent_id=%s&file_id=%s&redirect=true",
		"https://api.torbox.app/v1/api", url.QueryEscape(t.token), url.QueryEscape(id), url.QueryEscape(chosen.fileID))

	return ResolvedFile{Name: chosen.name, URL: dlURL}, nil
}

// pollCached polls mylist until the torrent has a non-empty file list,
// treating that as "ready" — same instant-cache-only philosophy as the
// other providers (see ErrNotCached).
func (t *TorBox) pollCached(ctx context.Context, id string) (json.RawMessage, error) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		var list torboxListResponse
		if err := t.c.get(ctx, "/torrents/mylist", url.Values{"id": {id}, "bypass_cache": {"true"}}, &list); err != nil {
			return nil, fmt.Errorf("torbox mylist: %w", err)
		}
		for _, item := range list.Data {
			itemID := firstNonEmptyString(item, "id", "torrent_id")
			if itemID != id && itemID != "" {
				continue
			}
			var files []json.RawMessage
			_ = json.Unmarshal(item, &struct {
				Files *[]json.RawMessage `json:"files"`
			}{Files: &files})
			if len(files) > 0 {
				return item, nil
			}
		}
		if time.Now().After(deadline) {
			return nil, ErrNotCached
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
