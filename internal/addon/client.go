package addon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client fetches manifests and resources from Stremio-protocol addons.
type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func transportURLFrom(manifestURL string) string {
	return strings.TrimSuffix(manifestURL, "/manifest.json")
}

func (c *Client) getJSON(ctx context.Context, reqURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s: unexpected status %s", reqURL, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", reqURL, err)
	}
	return nil
}

// FetchManifest fetches and parses an addon's manifest.json.
func (c *Client) FetchManifest(ctx context.Context, manifestURL string) (*Manifest, error) {
	var m Manifest
	if err := c.getJSON(ctx, manifestURL, &m); err != nil {
		return nil, err
	}
	m.TransportURL = transportURLFrom(manifestURL)
	return &m, nil
}

// EncodeExtra builds the "key=value&key2=value2" path segment the Stremio
// protocol appends to catalog requests for extra parameters (search, genre,
// skip, ...). Keys are sorted for deterministic output.
func EncodeExtra(extra map[string]string) string {
	if len(extra) == 0 {
		return ""
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		if extra[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(extra[k]))
	}
	return strings.Join(parts, "&")
}

// FetchCatalog fetches one catalog listing, optionally filtered/paginated
// via extra (e.g. {"search": "inception"} or {"skip": "100"}).
func (c *Client) FetchCatalog(ctx context.Context, m *Manifest, typ, id string, extra map[string]string) ([]MetaPreview, error) {
	reqURL := fmt.Sprintf("%s/catalog/%s/%s", m.TransportURL, typ, id)
	if e := EncodeExtra(extra); e != "" {
		reqURL += "/" + e
	}
	reqURL += ".json"

	var res struct {
		Metas []MetaPreview `json:"metas"`
	}
	if err := c.getJSON(ctx, reqURL, &res); err != nil {
		return nil, err
	}
	return res.Metas, nil
}

// FetchMeta fetches full metadata (including episode list for series) for
// one item.
func (c *Client) FetchMeta(ctx context.Context, m *Manifest, typ, id string) (*MetaDetail, error) {
	reqURL := fmt.Sprintf("%s/meta/%s/%s.json", m.TransportURL, typ, id)

	var res struct {
		Meta MetaDetail `json:"meta"`
	}
	if err := c.getJSON(ctx, reqURL, &res); err != nil {
		return nil, err
	}
	return &res.Meta, nil
}

// FetchStreams fetches the playable streams for one item or episode id
// (episode ids are already the full composite form, e.g. "tt123:1:2").
func (c *Client) FetchStreams(ctx context.Context, m *Manifest, typ, id string) ([]Stream, error) {
	reqURL := fmt.Sprintf("%s/stream/%s/%s.json", m.TransportURL, typ, id)

	var res struct {
		Streams []Stream `json:"streams"`
	}
	if err := c.getJSON(ctx, reqURL, &res); err != nil {
		return nil, err
	}
	return res.Streams, nil
}
