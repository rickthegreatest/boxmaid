// Command viewer shows a Mermaid diagram in a full-screen Bubble Tea
// program: go run ./examples/viewer diagram.mmd
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rickthegreatest/boxmaid/tui"
)

type model struct {
	d     tui.Model
	width int
	name  string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.d.SetSize(msg.Width, msg.Height-2)
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.d, cmd = m.d.Update(msg)
	return m, cmd
}

func (m model) View() string {
	x, y := m.d.Offset()
	help := fmt.Sprintf("%s · column %d, row %d\n↑↓/hjkl pan · PgUp/PgDn page · Home/End · q quit", m.name, x+1, y+1)
	return m.d.View() + "\n" + lipgloss.NewStyle().MaxWidth(m.width).Render(help)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: viewer diagram.mmd")
		os.Exit(2)
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	d := tui.New(string(src))
	if err := d.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := tea.NewProgram(model{d: d, name: os.Args[1]}, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
