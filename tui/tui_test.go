package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rickthegreatest/boxmaid"
)

func TestModelFitsAndPans(t *testing.T) {
	src := "flowchart LR\n  A[Collect the signals] --> B[Resolve them] --> C[Persist] --> D[Publish the outcome event] --> E[Notify]"
	m := New(src)
	m.SetSize(60, 10)
	if m.Err() != nil {
		t.Fatal(m.Err())
	}
	if w := m.Picture().W; w > 60 {
		t.Fatalf("the model should redraw to fit 60 columns, got %d", w)
	}
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "Collect") {
		t.Fatalf("view:\n%s", v)
	}
	if strings.Count(v, "\n") > 9 {
		t.Fatal("the view is taller than its size")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if ansi.Strip(m.View()) == v && m.Picture().H > 10 {
		t.Fatal("page down should scroll a tall diagram")
	}
}

func TestViewportSizeAndOptionChanges(t *testing.T) {
	m := New("graph LR\nA-->B")
	for _, size := range [][2]int{{0, 5}, {30, 0}, {-2, -1}} {
		m.SetSize(size[0], size[1])
		if got := m.View(); got != "" {
			t.Fatalf("zero viewport %v: %q", size, got)
		}
	}
	m.SetSize(30, 8)
	if strings.Count(m.View(), "\n") != 7 {
		t.Fatal("viewport must keep its allocated height")
	}
	m.SetOptions(boxmaid.Options{ASCII: true})
	if strings.Contains(m.View(), "┌") || !strings.Contains(m.View(), "+") {
		t.Fatalf("option change did not refresh the view: %s", m.View())
	}
	m.SetSource("not-a-diagram")
	if m.Err() == nil || ansi.StringWidth(strings.Split(m.View(), "\n")[0]) > 30 {
		t.Fatalf("error must be clipped to the viewport: %q", m.View())
	}
}
