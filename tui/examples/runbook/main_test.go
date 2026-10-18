package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/rickthegreatest/boxmaid"
)

func TestRunbookNavigationAndViewport(t *testing.T) {
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	for _, scene := range m.scenes {
		if _, err := boxmaid.Render(scene.source, boxmaid.Options{Width: 60}); err != nil {
			t.Fatalf("%s: %v", scene.name, err)
		}
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	view := ansi.Strip(next.View())
	if !strings.Contains(view, "Mermaid source") || !strings.Contains(view, "CLI->>API: GET /orders") {
		t.Fatalf("source navigation failed:\n%s", view)
	}
	for _, size := range [][2]int{{96, 28}, {54, 18}, {24, 10}} {
		next, _ = next.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(next.View(), "\n")
		if len(lines) > size[1] {
			t.Fatalf("view exceeds height %d", size[1])
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("view exceeds width %d: %q", size[0], line)
			}
		}
	}
}
