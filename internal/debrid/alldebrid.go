package debrid

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// AllDebrid implements Provider against api.alldebrid.com. Verified against
// AllDebrid's official docs (docs.alldebrid.com); note the "agent" query
// param some older integrations send was removed by AllDebrid in 2025 and
// is deliberately not sent here.
type AllDebrid struct {
	c *httpClient
}

func NewAllDebrid(apikey string) *AllDebrid {
	return &AllDebrid{c: newHTTPClient("https://api.alldebrid.com", apikey)}
}

func (a *AllDebrid) Name() string { return "AllDebrid" }

type adUploadResponse struct {
	Data struct {
		Magnets []struct {
			ID    int64  `json:"id"`
			Ready bool   `json:"ready"`
			Hash  string `json:"hash"`
		} `json:"magnets"`
	} `json:"data"`
}

type adStatusResponse struct {
	Data struct {
		Magnets struct {
			StatusCode int    `json:"statusCode"`
			Status     string `json:"status"`
		} `json:"magnets"`
	} `json:"data"`
}

// adFileNode models AllDebrid's abbreviated file-tree keys: n=name,
// s=size, l=direct link (leaf files only), e=child entries (folders).
type adFileNode struct {
	N string       `json:"n"`
	S int64        `json:"s"`
	L string       `json:"l"`
	E []adFileNode `json:"e"`
}

type adFilesResponse struct {
	Data struct {
		Magnets []struct {
			Files []adFileNode `json:"files"`
		} `json:"magnets"`
	} `json:"data"`
}

func (a *AllDebrid) Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (ResolvedFile, error) {
	magnet := magnetOrHash
	if len(magnet) == 40 || len(magnet) == 32 {
		magnet = MagnetFromHash(magnet)
	}

	var uploaded adUploadResponse
	if err := a.c.post(ctx, "/v4/magnet/upload", url.Values{"magnets[]": {magnet}}, &uploaded); err != nil {
		return ResolvedFile{}, fmt.Errorf("alldebrid magnet/upload: %w", err)
	}
	if len(uploaded.Data.Magnets) == 0 {
		return ResolvedFile{}, fmt.Errorf("alldebrid: no magnet returned from upload")
	}
	m := uploaded.Data.Magnets[0]

	if !m.Ready {
		ready, err := a.pollReady(ctx, m.ID)
		if err != nil {
			return ResolvedFile{}, err
		}
		if !ready {
			return ResolvedFile{}, ErrNotCached
		}
	}

	var files adFilesResponse
	if err := a.c.get(ctx, "/v4/magnet/files", url.Values{"id[]": {fmt.Sprintf("%d", m.ID)}}, &files); err != nil {
		return ResolvedFile{}, fmt.Errorf("alldebrid magnet/files: %w", err)
	}
	if len(files.Data.Magnets) == 0 {
		return ResolvedFile{}, fmt.Errorf("alldebrid: no files returned")
	}

	leaves := flattenAllDebridFiles(files.Data.Magnets[0].Files)
	if len(leaves) == 0 {
		return ResolvedFile{}, fmt.Errorf("alldebrid: torrent has no playable files")
	}

	chosen := pickFile(leaves, fileIdx, func(f adFileNode) int64 { return f.S })
	return ResolvedFile{Name: chosen.N, URL: chosen.L, Size: chosen.S}, nil
}

func flattenAllDebridFiles(nodes []adFileNode) []adFileNode {
	var leaves []adFileNode
	for _, n := range nodes {
		if len(n.E) > 0 {
			leaves = append(leaves, flattenAllDebridFiles(n.E)...)
			continue
		}
		if n.L != "" {
			leaves = append(leaves, n)
		}
	}
	return leaves
}

func (a *AllDebrid) pollReady(ctx context.Context, id int64) (bool, error) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		var status adStatusResponse
		if err := a.c.get(ctx, "/v4.1/magnet/status", url.Values{"id": {fmt.Sprintf("%d", id)}}, &status); err != nil {
			return false, fmt.Errorf("alldebrid magnet/status: %w", err)
		}
		s := status.Data.Magnets
		if s.StatusCode == 4 || strings.EqualFold(s.Status, "Ready") {
			return true, nil
		}
		if s.StatusCode >= 5 {
			return false, fmt.Errorf("alldebrid: magnet status %q", s.Status)
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
