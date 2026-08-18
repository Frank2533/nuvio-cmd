package tui

import (
	"fmt"
	"strings"

	"nuvio-cmd/internal/addon"
	"nuvio-cmd/internal/config"
	"nuvio-cmd/internal/debrid"
)

// addonState pairs a configured addon with its fetched manifest (or the
// error that occurred while fetching it).
type addonState struct {
	entry    config.AddonEntry
	manifest *addon.Manifest
	err      error
}

type addonItem struct{ state addonState }

func (i addonItem) Title() string {
	if i.state.manifest != nil {
		return i.state.manifest.Name
	}
	if !i.state.entry.Enabled {
		return i.state.entry.ManifestURL + " (disabled)"
	}
	if i.state.err != nil {
		return i.state.entry.ManifestURL + " (error)"
	}
	return i.state.entry.ManifestURL
}

func (i addonItem) Description() string {
	switch {
	case i.state.err != nil:
		return i.state.err.Error()
	case i.state.manifest != nil:
		return fmt.Sprintf("%s · %d catalog(s)", i.state.manifest.Description, len(i.state.manifest.Catalogs))
	default:
		return "disabled"
	}
}

func (i addonItem) FilterValue() string { return i.Title() }

// catalogItem is one browsable catalog, flattened across every active addon.
type catalogItem struct {
	addonName string
	manifest  *addon.Manifest
	catalog   addon.Catalog
}

func (i catalogItem) Title() string { return i.catalog.Name }
func (i catalogItem) Description() string {
	return fmt.Sprintf("%s · %s", i.addonName, i.catalog.Type)
}
func (i catalogItem) FilterValue() string { return i.Title() }

type metaItem struct{ meta addon.MetaPreview }

func (i metaItem) Title() string {
	if i.meta.ReleaseInfo != "" {
		return fmt.Sprintf("%s (%s)", i.meta.Name, i.meta.ReleaseInfo)
	}
	return i.meta.Name
}

func (i metaItem) Description() string {
	var parts []string
	if i.meta.IMDBRating != "" {
		parts = append(parts, "★ "+i.meta.IMDBRating)
	}
	if len(i.meta.Genres) > 0 {
		parts = append(parts, strings.Join(i.meta.Genres, ", "))
	}
	return strings.Join(parts, " · ")
}

func (i metaItem) FilterValue() string { return i.meta.Name }

type episodeItem struct{ video addon.Video }

func (i episodeItem) Title() string {
	title := i.video.Title
	if title == "" {
		title = fmt.Sprintf("Episode %d", i.video.Episode)
	}
	return fmt.Sprintf("S%02dE%02d · %s", i.video.Season, i.video.Episode, title)
}

func (i episodeItem) Description() string { return i.video.Released }
func (i episodeItem) FilterValue() string { return i.Title() }

// debridProviderItem is one of the fixed set of known debrid providers,
// paired with its configured entry (nil if not yet set up).
type debridProviderItem struct {
	id    string
	entry *config.DebridEntry
}

func (i debridProviderItem) Title() string { return debrid.DisplayName(i.id) }

func (i debridProviderItem) Description() string {
	if i.entry != nil && i.entry.Token != "" {
		return "configured"
	}
	return "not configured"
}

func (i debridProviderItem) FilterValue() string { return i.Title() }

type streamItem struct{ stream addon.Stream }

func (i streamItem) Title() string { return i.stream.DisplayTitle() }

func (i streamItem) Description() string {
	if i.stream.Playable() {
		return i.stream.Description
	}
	return "requires P2P streaming — not yet implemented (see plan M3)"
}

func (i streamItem) FilterValue() string { return i.Title() }
