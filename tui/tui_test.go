package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
