// Package tui implements Nuvio CMD's Bubble Tea terminal interface: an
// addon browser (Stremio protocol) feeding an mpv-backed player.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"nuvio-cmd/internal/addon"
	"nuvio-cmd/internal/config"
	"nuvio-cmd/internal/debrid"
	"nuvio-cmd/internal/player"
)

type screen int

const (
	screenMenu screen = iota
	screenAddons
	screenAddAddon
	screenDebrid
	screenDebridToken
	screenBrowseCatalogs
	screenBrowseItems
	screenDetails
	screenEpisodes
	screenStreams
	screenPlayer
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(1, 1, 0, 1)

	bodyStyle = lipgloss.NewStyle().Padding(0, 1)
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(1, 1, 0, 1)
)

type menuEntry struct{ title, desc string }

func (i menuEntry) Title() string       { return i.title }
func (i menuEntry) Description() string { return i.desc }
func (i menuEntry) FilterValue() string { return i.title }

// Model is the root Bubble Tea model for Nuvio CMD.
type Model struct {
	screen screen
	width  int
	height int

	client *addon.Client

	menu    list.Model
	addons  list.Model
	debrid  list.Model
	catalog list.Model
	items   list.Model
	episode list.Model
	streams list.Model

	addAddonInput    textinput.Model
	debridTokenInput textinput.Model
	spinner          spinner.Model
	loading          bool

	addonStates []addonState

	debridEntries         []config.DebridEntry
	debridManager         *debrid.Manager
	debridEditingProvider string

	// Navigation context for the currently selected chain
	activeCatalog  catalogItem
	activeMeta     *addon.MetaDetail
	activeManifest *addon.Manifest
	activeType     string
	streamsBackTo  screen

	player       *player.Player
	playerStatus player.Status
	playerTitle  string

	statusMsg string
	errMsg    string
}

func newList(title string) list.Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	// Filtering is off because it conflicts with the global q/esc
	// back-navigation shortcut used throughout the app; catalog search
	// (the addon protocol's own "search" extra param) is a separate,
	// not-yet-wired-up concern from bubbles' built-in list filter.
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	return l
}

func NewModel() Model {
	menu := newList("Nuvio CMD")
	menu.SetItems([]list.Item{
		menuEntry{title: "Browse", desc: "Browse catalogs from installed addons"},
		menuEntry{title: "Addons", desc: "Manage installed Stremio-protocol addons"},
		menuEntry{title: "Debrid", desc: "Configure debrid providers for instant-cache streaming"},
	})

	ti := textinput.New()
	ti.Placeholder = "https://example.com/manifest.json"
	ti.CharLimit = 256

	dti := textinput.New()
	dti.Placeholder = "API token"
	dti.CharLimit = 256
	dti.EchoMode = textinput.EchoPassword
	dti.EchoCharacter = '•'

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		screen:  screenMenu,
		client:  addon.NewClient(),
		menu:    menu,
		addons:  newList("Addons"),
		debrid:  newList("Debrid providers"),
		catalog: newList("Browse"),
		items:   newList("Catalog"),
		episode: newList("Episodes"),
		streams: newList("Streams"),

		addAddonInput:    ti,
		debridTokenInput: dti,
		spinner:          sp,
		loading:          true,
		debridManager:    debrid.NewManager(),
		statusMsg:        "unofficial · unaffiliated with NuvioMedia",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadAddonsCmd(m.client), loadDebridCmd())
}

func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h
	listH := h - 4
	for _, l := range []*list.Model{&m.menu, &m.addons, &m.debrid, &m.catalog, &m.items, &m.episode, &m.streams} {
		l.SetSize(w, listH)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
		return m, nil

	case addonsLoadedMsg:
		m.loading = false
		m.addonStates = msg.states
		m.refreshAddonsList()
		m.refreshCatalogsList()
		return m, nil

	case debridLoadedMsg:
		m.debridEntries = msg.entries
		m.debridManager = buildDebridManager(msg.entries)
		m.refreshDebridList()
		return m, nil

	case debridSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.debridEntries = msg.entries
		m.debridManager = buildDebridManager(msg.entries)
		m.refreshDebridList()
		m.screen = screenDebrid
		return m, nil

	case debridResolvedMsg:
		if msg.err != nil {
			m.loading = false
			m.errMsg = "debrid: " + msg.err.Error()
			return m, nil
		}
		m.loading = true
		m.playerTitle = msg.file.Name
		if m.playerTitle == "" {
			m.playerTitle = msg.title
		}
		return m, startPlayerCmd(msg.file.URL, m.playerTitle)

	case addonAddedMsg:
		m.loading = false
		if msg.state.err != nil {
			m.errMsg = msg.state.err.Error()
		} else {
			m.errMsg = ""
			m.addonStates = append(m.addonStates, msg.state)
			m.refreshAddonsList()
			m.refreshCatalogsList()
			m.screen = screenAddons
		}
		return m, nil

	case catalogLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.metas))
		for i, mp := range msg.metas {
			items[i] = metaItem{meta: mp}
		}
		m.items.SetItems(items)
		m.items.Title = m.activeCatalog.catalog.Name
		m.screen = screenBrowseItems
		return m, nil

	case metaLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.activeMeta = msg.meta
		if len(msg.meta.Videos) > 0 {
			epItems := make([]list.Item, len(msg.meta.Videos))
			for i, v := range msg.meta.Videos {
				epItems[i] = episodeItem{video: v}
			}
			m.episode.SetItems(epItems)
			m.episode.Title = "Episodes · " + msg.meta.Name
		}
		m.screen = screenDetails
		return m, nil

	case streamsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, len(msg.streams))
		for i, s := range msg.streams {
			items[i] = streamItem{stream: s}
		}
		m.streams.SetItems(items)
		m.screen = screenStreams
		return m, nil

	case playerStartedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.player = msg.p
		m.screen = screenPlayer
		return m, tea.Batch(waitPlayerCmd(msg.p), pollPlayerStatusCmd(msg.p))

	case playerStatusMsg:
		if m.player == nil {
			return m, nil
		}
		m.playerStatus = msg.status
		return m, pollPlayerStatusCmd(m.player)

	case playerExitedMsg:
		if m.player == nil {
			return m, nil
		}
		m.player = nil
		m.screen = screenStreams
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) refreshAddonsList() {
	items := make([]list.Item, len(m.addonStates))
	for i, s := range m.addonStates {
		items[i] = addonItem{state: s}
	}
	m.addons.SetItems(items)
}

func (m *Model) refreshDebridList() {
	items := make([]list.Item, 0, len(debrid.KnownProviders()))
	for _, id := range debrid.KnownProviders() {
		item := debridProviderItem{id: id}
		for _, e := range m.debridEntries {
			if e.Provider == id {
				item.entry = &e
				break
			}
		}
		items = append(items, item)
	}
	m.debrid.SetItems(items)
}

// buildDebridManager constructs the set of active Provider clients from
// persisted config, skipping disabled or blank-token entries.
func buildDebridManager(entries []config.DebridEntry) *debrid.Manager {
	var providers []debrid.Provider
	for _, e := range entries {
		if !e.Enabled || e.Token == "" {
			continue
		}
		p, err := debrid.New(e.Provider, e.Token)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}
	return debrid.NewManager(providers...)
}

// streamCapableAddons returns the manifests of enabled addons that both
// declare the "stream" resource and support typ — the set fetchStreamsCmd
// should query for a given movie/series.
func (m Model) streamCapableAddons(typ string) []*addon.Manifest {
	var out []*addon.Manifest
	for _, s := range m.addonStates {
		if s.manifest == nil || !s.manifest.HasResource("stream") {
			continue
		}
		for _, t := range s.manifest.Types {
			if t == typ {
				out = append(out, s.manifest)
				break
			}
		}
	}
	return out
}

func (m *Model) refreshCatalogsList() {
	var items []list.Item
	for _, s := range m.addonStates {
		if s.manifest == nil {
			continue
		}
		for _, c := range s.manifest.Catalogs {
			items = append(items, catalogItem{addonName: s.manifest.Name, manifest: s.manifest, catalog: c})
		}
	}
	m.catalog.SetItems(items)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit, except while typing into the add-addon text field or
	// while a player is running (there, 'q' has screen-specific meaning).
	if (key == "ctrl+c") || (key == "q" && m.screen == screenMenu) {
		return m, tea.Quit
	}

	// Back-navigation always clears any stale error from the screen being
	// left, so it doesn't linger into an unrelated one.
	if key == "esc" || (key == "q" && m.screen != screenMenu) {
		m.errMsg = ""
	}

	switch m.screen {
	case screenMenu:
		if key == "enter" {
			switch m.menu.SelectedItem().(menuEntry).title {
			case "Browse":
				m.screen = screenBrowseCatalogs
				return m, nil
			case "Addons":
				m.screen = screenAddons
				return m, nil
			case "Debrid":
				m.screen = screenDebrid
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd

	case screenAddons:
		switch key {
		case "esc", "q":
			m.screen = screenMenu
			return m, nil
		case "a":
			m.addAddonInput.SetValue("")
			m.addAddonInput.Focus()
			m.screen = screenAddAddon
			return m, textinput.Blink
		}
		var cmd tea.Cmd
		m.addons, cmd = m.addons.Update(msg)
		return m, cmd

	case screenAddAddon:
		switch key {
		case "esc":
			m.screen = screenAddons
			return m, nil
		case "enter":
			url := strings.TrimSpace(m.addAddonInput.Value())
			if url == "" {
				return m, nil
			}
			existing := make([]config.AddonEntry, len(m.addonStates))
			for i, s := range m.addonStates {
				existing[i] = s.entry
			}
			m.loading = true
			m.errMsg = ""
			return m, addAddonCmd(m.client, existing, url)
		}
		var cmd tea.Cmd
		m.addAddonInput, cmd = m.addAddonInput.Update(msg)
		return m, cmd

	case screenDebrid:
		switch key {
		case "esc", "q":
			m.screen = screenMenu
			return m, nil
		case "enter":
			if pi, ok := m.debrid.SelectedItem().(debridProviderItem); ok {
				m.debridEditingProvider = pi.id
				existing := ""
				if pi.entry != nil {
					existing = pi.entry.Token
				}
				m.debridTokenInput.SetValue(existing)
				m.debridTokenInput.Focus()
				m.screen = screenDebridToken
				return m, textinput.Blink
			}
		}
		var cmd tea.Cmd
		m.debrid, cmd = m.debrid.Update(msg)
		return m, cmd

	case screenDebridToken:
		switch key {
		case "esc":
			m.screen = screenDebrid
			return m, nil
		case "enter":
			token := strings.TrimSpace(m.debridTokenInput.Value())
			updated := make([]config.DebridEntry, 0, len(m.debridEntries)+1)
			found := false
			for _, e := range m.debridEntries {
				if e.Provider == m.debridEditingProvider {
					found = true
					if token != "" {
						updated = append(updated, config.DebridEntry{Provider: e.Provider, Token: token, Enabled: true})
					}
					continue
				}
				updated = append(updated, e)
			}
			if !found && token != "" {
				updated = append(updated, config.DebridEntry{Provider: m.debridEditingProvider, Token: token, Enabled: true})
			}
			m.loading = true
			m.errMsg = ""
			return m, saveDebridCmd(updated)
		}
		var cmd tea.Cmd
		m.debridTokenInput, cmd = m.debridTokenInput.Update(msg)
		return m, cmd

	case screenBrowseCatalogs:
		switch key {
		case "esc", "q":
			m.screen = screenMenu
			return m, nil
		case "enter":
			if ci, ok := m.catalog.SelectedItem().(catalogItem); ok {
				m.activeCatalog = ci
				m.activeManifest = ci.manifest
				m.activeType = ci.catalog.Type
				m.loading = true
				m.errMsg = ""
				return m, fetchCatalogCmd(m.client, ci.manifest, ci.catalog.Type, ci.catalog.ID, nil)
			}
		}
		var cmd tea.Cmd
		m.catalog, cmd = m.catalog.Update(msg)
		return m, cmd

	case screenBrowseItems:
		switch key {
		case "esc", "q":
			m.screen = screenBrowseCatalogs
			return m, nil
		case "enter":
			if mi, ok := m.items.SelectedItem().(metaItem); ok {
				m.loading = true
				m.errMsg = ""
				return m, fetchMetaCmd(m.client, m.activeManifest, m.activeType, mi.meta.ID)
			}
		}
		var cmd tea.Cmd
		m.items, cmd = m.items.Update(msg)
		return m, cmd

	case screenDetails:
		switch key {
		case "esc", "q":
			m.screen = screenBrowseItems
			return m, nil
		case "enter":
			if m.activeMeta != nil && len(m.activeMeta.Videos) > 0 {
				m.screen = screenEpisodes
				return m, nil
			}
			if m.activeMeta != nil {
				m.loading = true
				m.errMsg = ""
				m.streamsBackTo = screenDetails
				return m, fetchStreamsCmd(m.client, m.streamCapableAddons(m.activeType), m.activeType, m.activeMeta.ID)
			}
		}
		return m, nil

	case screenEpisodes:
		switch key {
		case "esc", "q":
			m.screen = screenDetails
			return m, nil
		case "enter":
			if ei, ok := m.episode.SelectedItem().(episodeItem); ok {
				m.loading = true
				m.errMsg = ""
				m.streamsBackTo = screenEpisodes
				return m, fetchStreamsCmd(m.client, m.streamCapableAddons(m.activeType), m.activeType, ei.video.ID)
			}
		}
		var cmd tea.Cmd
		m.episode, cmd = m.episode.Update(msg)
		return m, cmd

	case screenStreams:
		switch key {
		case "esc", "q":
			m.screen = m.streamsBackTo
			return m, nil
		case "enter":
			if si, ok := m.streams.SelectedItem().(streamItem); ok {
				if si.stream.Playable() {
					m.loading = true
					m.errMsg = ""
					m.playerTitle = si.stream.DisplayTitle()
					return m, startPlayerCmd(si.stream.URL, m.playerTitle)
				}
				if si.stream.InfoHash != "" && !m.debridManager.Empty() {
					m.loading = true
					m.errMsg = ""
					return m, resolveDebridCmd(m.debridManager, si.stream)
				}
				if si.stream.InfoHash != "" {
					m.errMsg = "this stream needs a debrid provider (add one under Debrid) or P2P streaming, which isn't implemented yet"
					return m, nil
				}
				m.errMsg = "this stream has no playable source"
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.streams, cmd = m.streams.Update(msg)
		return m, cmd

	case screenPlayer:
		switch key {
		case "q", "esc":
			if m.player != nil {
				p := m.player
				m.player = nil
				m.screen = screenStreams
				return m, stopPlayerCmd(p)
			}
		case " ":
			if m.player != nil {
				_ = m.player.TogglePause()
			}
		case "left":
			if m.player != nil {
				_ = m.player.Seek(-10)
			}
		case "right":
			if m.player != nil {
				_ = m.player.Seek(10)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	var body string

	switch m.screen {
	case screenMenu:
		body = m.menu.View()
	case screenAddons:
		body = m.addons.View() + hintStyle.Render("enter: view · a: add addon · esc: back")
	case screenAddAddon:
		body = headerStyle.Render("Add addon") + "\n" +
			bodyStyle.Render("Manifest URL:") + "\n" +
			bodyStyle.Render(m.addAddonInput.View()) +
			hintStyle.Render("enter: add · esc: cancel")
	case screenDebrid:
		body = m.debrid.View() + hintStyle.Render("enter: set token · esc: back")
	case screenDebridToken:
		body = headerStyle.Render("Debrid: "+debrid.DisplayName(m.debridEditingProvider)) + "\n" +
			bodyStyle.Render("API token:") + "\n" +
			bodyStyle.Render(m.debridTokenInput.View()) +
			hintStyle.Render("enter: save (blank clears it) · esc: cancel")
	case screenBrowseCatalogs:
		body = m.catalog.View() + hintStyle.Render("enter: open catalog · esc: back")
	case screenBrowseItems:
		body = m.items.View() + hintStyle.Render("enter: details · esc: back")
	case screenDetails:
		body = m.detailsView()
	case screenEpisodes:
		body = m.episode.View() + hintStyle.Render("enter: streams · esc: back")
	case screenStreams:
		body = m.streams.View() + hintStyle.Render("enter: play · esc: back")
	case screenPlayer:
		body = m.playerView()
	}

	if m.loading {
		body += "\n" + statusStyle.Render(m.spinner.View()+" loading...")
	}
	if m.errMsg != "" {
		body += "\n" + errStyle.Render("error: "+m.errMsg)
	}

	return fmt.Sprintf("%s\n%s", body, statusStyle.Render(m.statusMsg))
}

func (m Model) detailsView() string {
	if m.activeMeta == nil {
		return ""
	}
	meta := m.activeMeta
	var b strings.Builder
	b.WriteString(headerStyle.Render(meta.Name))
	b.WriteString("\n")
	if meta.IMDBRating != "" || meta.ReleaseInfo != "" {
		b.WriteString(bodyStyle.Render(fmt.Sprintf("★ %s · %s", meta.IMDBRating, meta.ReleaseInfo)))
		b.WriteString("\n")
	}
	if len(meta.Genres) > 0 {
		b.WriteString(bodyStyle.Render(strings.Join(meta.Genres, ", ")))
		b.WriteString("\n")
	}
	b.WriteString(bodyStyle.Render(meta.Description))
	b.WriteString("\n")
	if len(meta.Videos) > 0 {
		b.WriteString(hintStyle.Render(fmt.Sprintf("%d episode(s) · enter: browse episodes · esc: back", len(meta.Videos))))
	} else {
		b.WriteString(hintStyle.Render("enter: find streams · esc: back"))
	}
	return b.String()
}

func (m Model) playerView() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Now playing: " + m.playerTitle))
	b.WriteString("\n")
	b.WriteString(bodyStyle.Render(fmt.Sprintf("%s / %s%s",
		formatDuration(m.playerStatus.Position),
		formatDuration(m.playerStatus.Duration),
		pausedSuffix(m.playerStatus.Paused))))
	b.WriteString("\n")
	b.WriteString(bodyStyle.Render(progressBar(m.playerStatus.Position, m.playerStatus.Duration, 40)))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("mpv is playing in its own window · space: pause · ←/→: seek 10s · q: stop"))
	return b.String()
}

func pausedSuffix(paused bool) string {
	if paused {
		return " (paused)"
	}
	return ""
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func progressBar(pos, dur float64, width int) string {
	if dur <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	filled := int(float64(width) * pos / dur)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
}
