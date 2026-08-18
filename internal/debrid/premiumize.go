package debrid

import (
	"context"
	"fmt"
	"net/url"
)

// Premiumize implements Provider against www.premiumize.me/api. Verified
// against Premiumize's official API docs. Only already-cached torrents are
// resolved (via /cache/check + /transfer/directdl) — Premiumize's queued
// transfer flow for non-cached items is a much slower, separate mechanism
// this package intentionally doesn't use (see the package doc on
// ErrNotCached).
type Premiumize struct {
	c *httpClient
}

func NewPremiumize(apikey string) *Premiumize {
	return &Premiumize{c: newHTTPClient("https://www.premiumize.me/api", apikey)}
}

func (p *Premiumize) Name() string { return "Premiumize" }

type pmCacheCheckResponse struct {
	Response []bool `json:"response"`
}

type pmContentItem struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Link string `json:"link"`
}

type pmDirectDLResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Content []pmContentItem `json:"content"`
}

func (p *Premiumize) Resolve(ctx context.Context, magnetOrHash string, fileIdx *int) (ResolvedFile, error) {
	magnet := magnetOrHash
	if len(magnet) == 40 || len(magnet) == 32 {
		magnet = MagnetFromHash(magnet)
	}

	var cache pmCacheCheckResponse
	if err := p.c.get(ctx, "/cache/check", url.Values{"items[]": {magnet}}, &cache); err != nil {
		return ResolvedFile{}, fmt.Errorf("premiumize cache/check: %w", err)
	}
	if len(cache.Response) == 0 || !cache.Response[0] {
		return ResolvedFile{}, ErrNotCached
	}

	var dl pmDirectDLResponse
	if err := p.c.post(ctx, "/transfer/directdl", url.Values{"src": {magnet}}, &dl); err != nil {
		return ResolvedFile{}, fmt.Errorf("premiumize transfer/directdl: %w", err)
	}
	if dl.Status != "success" {
		return ResolvedFile{}, fmt.Errorf("premiumize: %s", dl.Message)
	}
	if len(dl.Content) == 0 {
		return ResolvedFile{}, fmt.Errorf("premiumize: no playable content returned")
	}

	chosen := pickFile(dl.Content, fileIdx, func(c pmContentItem) int64 { return c.Size })
	return ResolvedFile{Name: chosen.Path, URL: chosen.Link, Size: chosen.Size}, nil
}
