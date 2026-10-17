// Package tui shows boxmaid diagrams in Bubble Tea programs: Model is a
// pannable diagram view, and Lines styles a picture with lipgloss for any
// other layout.
//
//	m := tui.New(src)
//	m.SetSize(width, height)
//	… in Update: m, cmd = m.Update(msg)
//	… in View:   m.View()
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/boxmaid/boxmaid"
)

// Styles maps roles to lipgloss styles.
type Styles map[boxmaid.Role]lipgloss.Style

// DefaultStyles follow the terminal's background, dark or light.
func DefaultStyles() Styles {
	c := func(light, dark string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: light, Dark: dark})
	}
	return Styles{
		boxmaid.RoleTitle:        lipgloss.NewStyle().Bold(true),
		boxmaid.RoleNode:         c("#3b5bdb", "#748ffc"),
		boxmaid.RoleNodeText:     lipgloss.NewStyle().Bold(true),
		boxmaid.RoleDecision:     c("#e67700", "#ffc078"),
		boxmaid.RoleStart:        lipgloss.NewStyle().Bold(true),
		boxmaid.RoleEdge:         c("#868e96", "#5c6370"),
		boxmaid.RoleArrow:        lipgloss.NewStyle().Bold(true),
		boxmaid.RoleEdgeLabel:    c("#0c8599", "#66d9e8"),
		boxmaid.RoleCluster:      c("#adb5bd", "#4b5263"),
		boxmaid.RoleClusterTitle: c("#862e9c", "#e599f7").Bold(true),
		boxmaid.RoleNote:         c("#e67700", "#ffd43b"),
		boxmaid.RoleActor:        c("#3b5bdb", "#748ffc"),
		boxmaid.RoleLifeline:     c("#adb5bd", "#4b5263"),
		boxmaid.RoleBlock:        c("#868e96", "#5c6370"),
		boxmaid.RoleBlockTitle:   c("#0c8599", "#66d9e8").Bold(true),
		boxmaid.RoleBar:          c("#3b5bdb", "#748ffc"),
		boxmaid.RoleMuted:        c("#868e96", "#7f8790"),
		boxmaid.RoleAccent:       c("#0c8599", "#66d9e8").Bold(true),
		boxmaid.RoleWarn:         c("#c92a2a", "#ff8787").Bold(true),
		boxmaid.RoleGood:         c("#2b8a3e", "#8ce99a"),
	}
}

// Lines is the picture styled, one string per row: drop them into any
// lipgloss layout. The diagram's own colours (classDef, style) win over
// the role's.
func Lines(p *boxmaid.Picture, s Styles) []string {
	out := make([]string, len(p.Cells))
	for y, row := range p.Cells {
		var b strings.Builder
		var run strings.Builder
		var cur lipgloss.Style
		key := "\x00"
		flush := func() {
			if run.Len() > 0 {
				b.WriteString(cur.Render(run.String()))
				run.Reset()
			}
		}
		for _, c := range row {
			if c.S == "" {
				continue // the right half of a wide character
			}
			k := ""
			if c.S != " " {
				k = string(rune('a'+c.Role)) + c.Color
				if c.Bold {
					k += "!"
				}
			}
			if k != key {
				flush()
				key = k
				cur = lipgloss.NewStyle()
				if k != "" {
					cur = s[c.Role]
					if c.Color != "" {
						cur = cur.Foreground(lipgloss.Color(c.Color))
					}
					if c.Bold {
						cur = cur.Bold(true)
					}
				}
			}
			run.WriteString(c.S)
		}
		flush()
		out[y] = strings.TrimRight(b.String(), " ")
	}
	return out
}

// KeyMap names the keys Model pans with.
type KeyMap struct {
	Up, Down, Left, Right, PageUp, PageDown, Home, End []string
}

// DefaultKeyMap: arrows and vim keys.
var DefaultKeyMap = KeyMap{
	Up: []string{"up", "k"}, Down: []string{"down", "j"}, Left: []string{"left", "h"}, Right: []string{"right", "l"},
	PageUp: []string{"pgup", "ctrl+u"}, PageDown: []string{"pgdown", "ctrl+d", " "}, Home: []string{"home", "g"}, End: []string{"end", "G"},
}

// Model is a diagram you can pan around. It redraws to fit when its width
// changes (wrapping labels, turning wide flowcharts), and pans with the
// keys and the mouse wheel when the result is still larger than the view.
type Model struct {
	Styles  Styles
	KeyMap  KeyMap
	Options boxmaid.Options // Width is managed by the model

	src        string
	pic        *boxmaid.Picture
	lines      []string
	err        error
	w, h, x, y int
}

// New makes a view of the diagram in src.
func New(src string) Model {
	m := Model{Styles: DefaultStyles(), KeyMap: DefaultKeyMap}
	m.SetSource(src)
	return m
}

// SetSource shows another diagram.
func (m *Model) SetSource(src string) {
	m.src, m.x, m.y = src, 0, 0
	m.render()
}

// SetSize sets the view's size; the diagram is redrawn to fit the width.
func (m *Model) SetSize(w, h int) {
	if w != m.w {
		m.w, m.h = w, h
		m.render()
		return
	}
	m.h = h
	m.clamp()
}

// Err is why the diagram couldn't be drawn, if it couldn't.
func (m Model) Err() error { return m.err }

// Picture is the drawn diagram (nil when it couldn't be drawn).
func (m Model) Picture() *boxmaid.Picture { return m.pic }

func (m *Model) render() {
	opt := m.Options
	opt.Width = m.w
	m.pic, m.err = boxmaid.Render(m.src, opt)
	m.lines = nil
	if m.err == nil {
		m.lines = Lines(m.pic, m.Styles)
	}
	m.clamp()
}

func (m *Model) clamp() {
	if m.pic == nil {
		return
	}
	m.x = max(0, min(m.x, m.pic.W-m.w))
	m.y = max(0, min(m.y, m.pic.H-m.h))
}

func has(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

// Init does nothing; the model needs no commands.
func (m Model) Init() tea.Cmd { return nil }

// Update pans with the keys and the mouse wheel, and follows window sizes
// when the model fills the window.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		switch {
		case has(m.KeyMap.Up, k):
			m.y--
		case has(m.KeyMap.Down, k):
			m.y++
		case has(m.KeyMap.Left, k):
			m.x -= 4
		case has(m.KeyMap.Right, k):
			m.x += 4
		case has(m.KeyMap.PageUp, k):
			m.y -= max(1, m.h-1)
		case has(m.KeyMap.PageDown, k):
			m.y += max(1, m.h-1)
		case has(m.KeyMap.Home, k):
			m.x, m.y = 0, 0
		case has(m.KeyMap.End, k):
			m.y = 1 << 30
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.y -= 3
		case tea.MouseButtonWheelDown:
			m.y += 3
		case tea.MouseButtonWheelLeft:
			m.x -= 4
		case tea.MouseButtonWheelRight:
			m.x += 4
		}
	}
	m.clamp()
	return m, nil
}

// View is the visible part of the diagram.
func (m Model) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(m.err.Error())
	}
	if m.pic == nil {
		return ""
	}
	var out []string
	for i := m.y; i < min(len(m.lines), m.y+max(1, m.h)); i++ {
		out = append(out, cut(m.lines[i], m.x, m.w))
	}
	return strings.Join(out, "\n")
}

// cut keeps w cells of a styled line, starting at cell x.
func cut(s string, x, w int) string {
	if x == 0 && (w <= 0 || lipgloss.Width(s) <= w) {
		return s
	}
	return truncateANSI(dropANSI(s, x), w)
}
