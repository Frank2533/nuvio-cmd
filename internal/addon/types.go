// Package addon implements a client for the Stremio addon protocol
// (https://github.com/Stremio/stremio-addon-sdk) — the open catalog/meta/
// stream protocol NuvioDesktop's addon system is also built on. Shapes here
// are verified against a live addon (v3-cinemeta.strem.io), not guessed.
package addon

import "encoding/json"

// Resource describes one capability an addon exposes. The wire format is
// either a bare string ("catalog") or an object with type/idPrefix
// filtering — real addons use both forms, so it unmarshals either.
type Resource struct {
	Name       string
	Types      []string
	IDPrefixes []string
}

func (r *Resource) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		r.Name = name
		return nil
	}
	var obj struct {
		Name       string   `json:"name"`
		Types      []string `json:"types"`
		IDPrefixes []string `json:"idPrefixes"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.Name, r.Types, r.IDPrefixes = obj.Name, obj.Types, obj.IDPrefixes
	return nil
}

// ExtraProperty describes one extra query parameter a catalog accepts
// (e.g. "search", "genre", "skip").
type ExtraProperty struct {
	Name         string   `json:"name"`
	IsRequired   bool     `json:"isRequired,omitempty"`
	Options      []string `json:"options,omitempty"`
	OptionsLimit int      `json:"optionsLimit,omitempty"`
}

// Catalog describes one browsable catalog an addon serves for a given type.
type Catalog struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Extra  []ExtraProperty `json:"extra,omitempty"`
	Genres []string        `json:"genres,omitempty"`

	// Legacy/alternate shape some addons still emit instead of Extra.
	ExtraSupported []string `json:"extraSupported,omitempty"`
	ExtraRequired  []string `json:"extraRequired,omitempty"`
}

// NormalizedExtra returns the extra-property list, synthesizing it from the
// legacy extraSupported/extraRequired string arrays when an addon doesn't
// provide the newer structured "extra" field.
func (c Catalog) NormalizedExtra() []ExtraProperty {
	if len(c.Extra) > 0 {
		return c.Extra
	}
	if len(c.ExtraSupported) == 0 {
		return nil
	}
	required := make(map[string]bool, len(c.ExtraRequired))
	for _, n := range c.ExtraRequired {
		required[n] = true
	}
	out := make([]ExtraProperty, 0, len(c.ExtraSupported))
	for _, n := range c.ExtraSupported {
		out = append(out, ExtraProperty{Name: n, IsRequired: required[n]})
	}
	return out
}

func (c Catalog) SupportsSearch() bool {
	for _, e := range c.NormalizedExtra() {
		if e.Name == "search" {
			return true
		}
	}
	return false
}

// Manifest is an addon's self-description, fetched from
// {transportUrl}/manifest.json.
type Manifest struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Version     string     `json:"version"`
	Logo        string     `json:"logo,omitempty"`
	Resources   []Resource `json:"resources"`
	Types       []string   `json:"types"`
	IDPrefixes  []string   `json:"idPrefixes,omitempty"`
	Catalogs    []Catalog  `json:"catalogs,omitempty"`

	// TransportURL is not part of the JSON wire format: it's the manifest
	// URL with the trailing "/manifest.json" removed, and is the base every
	// other resource request is built from. Set by Client.FetchManifest.
	TransportURL string `json:"-"`
}

func (m Manifest) HasResource(name string) bool {
	for _, r := range m.Resources {
		if r.Name == name {
			return true
		}
	}
	return false
}

// MetaPreview is the summary shape returned in catalog listings.
type MetaPreview struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Poster      string   `json:"poster,omitempty"`
	Background  string   `json:"background,omitempty"`
	Description string   `json:"description,omitempty"`
	ReleaseInfo string   `json:"releaseInfo,omitempty"`
	Year        string   `json:"year,omitempty"`
	IMDBRating  string   `json:"imdbRating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Runtime     string   `json:"runtime,omitempty"`
}

// Video is one playable entry within a series (an episode) or a multi-part
// meta. Its ID is already the full composite id ("tt123:1:2") used to query
// the stream resource directly.
type Video struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Season   int    `json:"season,omitempty"`
	Episode  int    `json:"episode,omitempty"`
	Released string `json:"released,omitempty"`
	Overview string `json:"overview,omitempty"`
}

// MetaDetail is the full shape returned from {transportUrl}/meta/....json.
type MetaDetail struct {
	MetaPreview
	Videos []Video `json:"videos,omitempty"`
}

// Subtitle is an out-of-band subtitle track offered alongside a stream.
type Subtitle struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Lang string `json:"lang"`
}

// Stream is one playable option returned from the stream resource. Only
// direct-URL streams are playable in M1 — infoHash (magnet/torrent) streams
// need the P2P engine, which is a later milestone.
type Stream struct {
	URL         string     `json:"url,omitempty"`
	YouTubeID   string     `json:"ytId,omitempty"`
	InfoHash    string     `json:"infoHash,omitempty"`
	FileIdx     *int       `json:"fileIdx,omitempty"`
	Name        string     `json:"name,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Subtitles   []Subtitle `json:"subtitles,omitempty"`
}

// Playable reports whether Nuvio CMD can hand this stream to mpv directly.
func (s Stream) Playable() bool {
	return s.URL != ""
}

// DisplayTitle returns the best human-readable label for this stream.
func (s Stream) DisplayTitle() string {
	switch {
	case s.Title != "":
		return s.Title
	case s.Name != "":
		return s.Name
	case s.InfoHash != "":
		return "torrent: " + s.InfoHash
	default:
		return s.URL
	}
}
