// Command nuvio is the entrypoint for Nuvio CMD, an unofficial terminal
// media client. See /home/haak4003/.claude/plans for the project plan.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"nuvio-cmd/internal/tui"
)

func main() {
	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nuvio: fatal:", err)
		os.Exit(1)
	}
}
