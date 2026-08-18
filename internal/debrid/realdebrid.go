package debrid

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// RealDebrid implements Provider against api.real-debrid.com/rest/1.0.
// Verified against Real-Debrid's official API docs.
type RealDebrid struct {
	c *httpClient
}

func NewRealDebrid(token string) *RealDebrid {
	return &RealDebrid{c: newHTTPClient("https://api.real-debrid.com/rest/1.0", token)}
}

func (r *RealDebrid) Name() string { return "Real-Debrid" }

type rdFile struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Selected int    `json:"selected"`
}

type rdTorrentInfo struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Files  []rdFile `json:"files"`
	Links  []string `json:"links"`
}

func (r *RealDebrid) Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (ResolvedFile, error) {
	magnet := magnetOrHash
	if len(magnet) == 40 || len(magnet) == 32 { // bare infoHash, not a magnet URI
		magnet = MagnetFromHash(magnet)
	}

	var added struct {
		ID string `json:"id"`
	}
	if err := r.c.post(ctx, "/torrents/addMagnet", url.Values{"magnet": {magnet}}, &added); err != nil {
		return ResolvedFile{}, fmt.Errorf("real-debrid addMagnet: %w", err)
	}

	info, err := r.getInfo(ctx, added.ID)
	if err != nil {
		return ResolvedFile{}, err
	}

	selectID := "all"
	var wantPath string
	if fileIdx != nil && *fileIdx >= 0 && *fileIdx < len(info.Files) {
		selectID = fmt.Sprintf("%d", info.Files[*fileIdx].ID)
		wantPath = info.Files[*fileIdx].Path
	}
	if err := r.c.post(ctx, "/torrents/selectFiles/"+added.ID, url.Values{"files": {selectID}}, nil); err != nil {
		return ResolvedFile{}, fmt.Errorf("real-debrid selectFiles: %w", err)
	}

	info, err = r.pollUntilDownloaded(ctx, added.ID)
	if err != nil {
		return ResolvedFile{}, err
	}
	if len(info.Links) == 0 {
		return ResolvedFile{}, fmt.Errorf("real-debrid: no links returned for torrent %s", added.ID)
	}

	var unrestricted struct {
		Download string `json:"download"`
		Filename string `json:"filename"`
		Filesize int64  `json:"filesize"`
	}
	if err := r.c.post(ctx, "/unrestrict/link", url.Values{"link": {info.Links[0]}}, &unrestricted); err != nil {
		return ResolvedFile{}, fmt.Errorf("real-debrid unrestrict: %w", err)
	}

	name := unrestricted.Filename
	if wantPath != "" {
		name = wantPath
	}
	return ResolvedFile{Name: name, URL: unrestricted.Download, Size: unrestricted.Filesize}, nil
}

func (r *RealDebrid) getInfo(ctx context.Context, id string) (rdTorrentInfo, error) {
	var info rdTorrentInfo
	if err := r.c.get(ctx, "/torrents/info/"+id, nil, &info); err != nil {
		return rdTorrentInfo{}, fmt.Errorf("real-debrid torrents/info: %w", err)
	}
	return info, nil
}

// pollUntilDownloaded polls for up to ~30s: Real-Debrid resolves near
// instantly when the torrent is already cached (the common case this
// package targets), and ErrNotCached is returned otherwise rather than
// waiting for a real download.
func (r *RealDebrid) pollUntilDownloaded(ctx context.Context, id string) (rdTorrentInfo, error) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		info, err := r.getInfo(ctx, id)
		if err != nil {
			return rdTorrentInfo{}, err
		}
		switch info.Status {
		case "downloaded":
			return info, nil
		case "magnet_error", "error", "virus", "dead":
			return rdTorrentInfo{}, fmt.Errorf("real-debrid: torrent status %q", info.Status)
		}
		if time.Now().After(deadline) {
			return rdTorrentInfo{}, ErrNotCached
		}
		select {
		case <-ctx.Done():
			return rdTorrentInfo{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
