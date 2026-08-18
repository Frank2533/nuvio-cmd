package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)
)

type menuItem struct {
	title, desc string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// Model is the root bubbletea model for Nuvio CMD. Milestone M0 only wires
// up navigation chrome; each menu entry becomes a real screen in later
// milestones (see the plan's M1+ subsystems).
type Model struct {
	list   list.Model
	status string
}

func NewModel() Model {
	items := []list.Item{
		menuItem{title: "Browse", desc: "Browse catalogs from installed addons (M1)"},
		menuItem{title: "Search", desc: "Search across addons and TMDB metadata (M1)"},
		menuItem{title: "Library", desc: "Watch progress, collections, profiles (M4)"},
		menuItem{title: "Downloads", desc: "Resumable download queue (M5)"},
		menuItem{title: "Addons", desc: "Manage installed Stremio-protocol addons (M1)"},
		menuItem{title: "Settings", desc: "Debrid providers, tracking accounts, player config"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Nuvio CMD"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return Model{
		list:   l,
		status: "unofficial · unaffiliated with NuvioMedia · M0 scaffold",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := 4, 4
		m.list.SetSize(msg.Width-h, msg.Height-v)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return fmt.Sprintf("%s\n%s", m.list.View(), statusStyle.Render(m.status))
}
