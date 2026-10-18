// Command runbook embeds Boxmaid in a small, keyboard-driven service runbook.
// Run from the repository root: go run ./tui/examples/runbook
package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rickthegreatest/boxmaid/tui"
)

//go:embed diagrams/*.mmd
var diagrams embed.FS

type scene struct{ name, description, source string }

type model struct {
	scenes        []scene
	selected      int
	source        bool
	width, height int
	diagram       tui.Model
}

func newModel() (model, error) {
	m := model{}
	for _, entry := range []struct{ file, name, description string }{
		{"checkout", "Checkout", "Follow the stock decision before reserving an order."},
		{"request", "Request", "Trace a client request and its response."},
		{"services", "Services", "Inspect the gateway, worker, and order store."},
	} {
		data, err := diagrams.ReadFile("diagrams/" + entry.file + ".mmd")
		if err != nil {
			return model{}, err
		}
		m.scenes = append(m.scenes, scene{entry.name, entry.description, strings.TrimSpace(string(data))})
	}
	m.diagram = tui.New(m.scenes[0].source)
	return m, m.diagram.Err()
}

func (m model) Init() tea.Cmd { return nil }

func (m model) dimensions() (width, height int) {
	width = max(0, m.width-4)
	if m.width >= 76 {
		width = max(0, width-24)
	}
	return width, max(0, m.height-7)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.diagram.SetSize(m.dimensions())
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "s":
			m.source = !m.source
			return m, nil
		case "1", "2", "3":
			m.selected, m.source = int(key[0]-'1'), false
			m.diagram.SetSource(m.scenes[m.selected].source)
			return m, nil
		}
	}
	if m.source {
		return m, nil
	}
	var cmd tea.Cmd
	m.diagram, cmd = m.diagram.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.width <= 4 || m.height <= 0 {
		return ""
	}
	width, height := m.dimensions()
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#5eead4")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#a3b4ca"))
	current := m.scenes[m.selected]
	header := accent.Render("BOXMAID") + "  /  SERVICE RUNBOOK"
	subtitle := muted.Render(current.description)
	body := m.diagram.View()
	mode := "drawing"
	if m.source {
		mode = "Mermaid source"
		lines := make([]string, height)
		for i, line := range strings.Split(current.source, "\n") {
			if i >= height {
				break
			}
			lines[i] = ansi.Truncate(muted.Render(fmt.Sprintf("%2d  ", i+1))+line, width, "")
		}
		body = strings.Join(lines, "\n")
	}
	if m.width >= 76 {
		var items []string
		items = append(items, muted.Render("DIAGRAMS"), "")
		for i, entry := range m.scenes {
			item := fmt.Sprintf("%d  %s", i+1, entry.name)
			if i == m.selected {
				item = accent.Render("› " + item)
			} else {
				item = "  " + item
			}
			items = append(items, item, "")
		}
		sidebar := lipgloss.NewStyle().Width(22).MaxHeight(height).Render(strings.Join(items, "\n"))
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", body)
	}
	status := fmt.Sprintf("%s · %s · %d-column budget", current.name, mode, width)
	help := "1–3 choose · s source/drawing · arrows/hjkl pan · q quit"
	clip := func(s string) string { return ansi.Truncate(s, max(0, m.width-4), "") }
	view := clip(header) + "\n" + clip(subtitle) + "\n\n" + body + "\n\n" + clip(muted.Render(status)) + "\n" + clip(muted.Render(help))
	return lipgloss.NewStyle().Padding(0, 2).MaxWidth(m.width).MaxHeight(m.height).Render(view)
}

func main() {
	m, err := newModel()
	if err == nil {
		_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
