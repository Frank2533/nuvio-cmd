package debrid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClient is the shared request plumbing every provider uses: all four
// (Real-Debrid, AllDebrid, Premiumize, TorBox) authenticate the same way,
// via a "Authorization: Bearer <token>" header.
type httpClient struct {
	base  string
	token string
	http  *http.Client
}

func newHTTPClient(base, token string) *httpClient {
	return &httpClient{base: base, token: token, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *httpClient) do(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	fullURL := c.base + path
	if method == http.MethodGet && form != nil {
		fullURL += "?" + form.Encode()
	} else if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", fullURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("request %s: status %s: %s", fullURL, resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", fullURL, err)
	}
	return nil
}

func (c *httpClient) get(ctx context.Context, path string, form url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, form, out)
}

func (c *httpClient) post(ctx context.Context, path string, form url.Values, out any) error {
	return c.do(ctx, http.MethodPost, path, form, out)
}

// firstNonEmptyString probes a JSON object for the first of several
// candidate keys with a non-empty value, accepting either a JSON string or
// a JSON number (IDs frequently come back as numbers). Used where a
// provider's exact field naming isn't fully certain from documentation
// (see TorBox).
func firstNonEmptyString(raw json.RawMessage, keys ...string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
		var n json.Number
		if err := json.Unmarshal(v, &n); err == nil && n != "" {
			return n.String()
		}
	}
	return ""
}
