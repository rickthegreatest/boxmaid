package boxmaid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Colouring a picture for a terminal.

// Style is how cells of one role look: a colour (an ANSI colour number
// 0–255, or "#rrggbb") and bold.
type Style struct {
	Color string
	Bold  bool
}

// Theme maps roles to styles. TrueColor lets the diagram's own colours
// (classDef, style) through; without it they're ignored, so a theme made of
// the 16 basic colours works in every terminal.
type Theme struct {
	Name      string
	Styles    map[Role]Style
	TrueColor bool
}

// Built-in themes. DefaultTheme uses the terminal's own 16 colours, so it
// follows the user's colour scheme, dark or light.
var (
	DefaultTheme = Theme{Name: "default", Styles: map[Role]Style{
		RoleTitle: {"", true}, RoleNode: {"4", false}, RoleNodeText: {"", true}, RoleDecision: {"3", false},
		RoleStart: {"", true}, RoleEdge: {"8", false}, RoleArrow: {"", true}, RoleEdgeLabel: {"6", false},
		RoleCluster: {"8", false}, RoleClusterTitle: {"5", true}, RoleNote: {"3", false}, RoleActor: {"4", false},
		RoleLifeline: {"8", false}, RoleBlock: {"8", false}, RoleBlockTitle: {"6", true}, RoleBar: {"4", false},
		RoleMuted: {"8", false}, RoleAccent: {"6", true}, RoleWarn: {"1", true}, RoleGood: {"2", false},
	}}
	// MonoTheme has no colour at all, only bold for names and titles.
	MonoTheme = Theme{Name: "mono", Styles: map[Role]Style{
		RoleTitle: {"", true}, RoleNodeText: {"", true}, RoleClusterTitle: {"", true}, RoleBlockTitle: {"", true},
		RoleArrow: {"", true}, RoleWarn: {"", true},
	}}
	// DraculaTheme and NordTheme are true-colour themes after the popular
	// editor themes.
	DraculaTheme = Theme{Name: "dracula", TrueColor: true, Styles: map[Role]Style{
		RoleTitle: {"#f8f8f2", true}, RoleNode: {"#bd93f9", false}, RoleNodeText: {"#f8f8f2", true}, RoleDecision: {"#ffb86c", false},
		RoleStart: {"#f8f8f2", true}, RoleEdge: {"#6272a4", false}, RoleArrow: {"#f8f8f2", true}, RoleEdgeLabel: {"#8be9fd", false},
		RoleCluster: {"#6272a4", false}, RoleClusterTitle: {"#ff79c6", true}, RoleNote: {"#f1fa8c", false}, RoleActor: {"#bd93f9", false},
		RoleLifeline: {"#6272a4", false}, RoleBlock: {"#6272a4", false}, RoleBlockTitle: {"#8be9fd", true}, RoleBar: {"#bd93f9", false},
		RoleMuted: {"#6272a4", false}, RoleAccent: {"#8be9fd", true}, RoleWarn: {"#ff5555", true}, RoleGood: {"#50fa7b", false},
	}}
	NordTheme = Theme{Name: "nord", TrueColor: true, Styles: map[Role]Style{
		RoleTitle: {"#eceff4", true}, RoleNode: {"#81a1c1", false}, RoleNodeText: {"#eceff4", true}, RoleDecision: {"#ebcb8b", false},
		RoleStart: {"#eceff4", true}, RoleEdge: {"#4c566a", false}, RoleArrow: {"#d8dee9", true}, RoleEdgeLabel: {"#88c0d0", false},
		RoleCluster: {"#4c566a", false}, RoleClusterTitle: {"#b48ead", true}, RoleNote: {"#ebcb8b", false}, RoleActor: {"#81a1c1", false},
		RoleLifeline: {"#4c566a", false}, RoleBlock: {"#4c566a", false}, RoleBlockTitle: {"#88c0d0", true}, RoleBar: {"#5e81ac", false},
		RoleMuted: {"#616e88", false}, RoleAccent: {"#88c0d0", true}, RoleWarn: {"#bf616a", true}, RoleGood: {"#a3be8c", false},
	}}
)

// Themes are the built-in themes by name.
var Themes = map[string]Theme{"default": DefaultTheme, "mono": MonoTheme, "dracula": DraculaTheme, "nord": NordTheme}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// CellStyle resolves a cell's role, explicit colour and emphasis. Output
// adapters should use this to keep the same precedence as ANSI rendering.
func (t Theme) CellStyle(c Cell) Style {
	st := t.Styles[c.Role]
	if c.Color != "" && t.TrueColor {
		st.Color = c.Color
	}
	st.Bold = st.Bold || c.Bold
	return st
}

// sgr is the escape sequence that switches to st ("" for none).
func sgr(st Style, trueColor bool) string {
	var codes []string
	if st.Bold {
		codes = append(codes, "1")
	}
	switch c := st.Color; {
	case hexColor.MatchString(c) && trueColor:
		r, _ := strconv.ParseUint(c[1:3], 16, 8)
		g, _ := strconv.ParseUint(c[3:5], 16, 8)
		b, _ := strconv.ParseUint(c[5:7], 16, 8)
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
	case c != "" && !strings.HasPrefix(c, "#"):
		n, err := strconv.Atoi(c)
		switch {
		case err != nil || n < 0 || n > 255:
		case n < 8:
			codes = append(codes, strconv.Itoa(30+n))
		case n < 16:
			codes = append(codes, strconv.Itoa(90+n-8))
		default:
			codes = append(codes, "38;5;"+strconv.Itoa(n))
		}
	}
	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

// ANSI is the picture coloured with the theme, as text for a terminal.
func (p *Picture) ANSI(t Theme) string {
	lines := make([]string, len(p.Cells))
	for y, row := range p.Cells {
		var b strings.Builder
		cur := ""
		end := len(row)
		for end > 0 && (row[end-1].S == " " || row[end-1].S == "") && !row[end-1].wide {
			end--
		}
		for _, c := range row[:end] {
			if c.wide {
				continue
			}
			st := t.CellStyle(c)
			code := ""
			if c.S != " " {
				code = sgr(st, t.TrueColor)
			}
			if code != cur {
				if cur != "" {
					b.WriteString("\x1b[0m")
				}
				b.WriteString(code)
				cur = code
			}
			b.WriteString(c.S)
		}
		if cur != "" {
			b.WriteString("\x1b[0m")
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}
