// Package boxmaid draws Mermaid diagrams for the terminal: boxes, lines and
// charts from Unicode box drawing (or plain ASCII), with layered layout
// and best-effort width fitting.
//
//	pic, err := boxmaid.Render(src, boxmaid.Options{Width: 100})
//	fmt.Println(pic)                          // plain text
//	fmt.Println(pic.ANSI(boxmaid.DefaultTheme)) // coloured
//
// Render returns a Picture: a grid of cells, each with a Role (node border,
// edge, label…) so a TUI can colour it with its own theme. Diagrams that
// can't be drawn return an *UnsupportedError; LiveURL links any diagram to
// mermaid.live.
//
// boxmaid has no dependencies. Its Rust twin (the boxmaid crate) produces
// the same output for the same input.
package boxmaid

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Options shape the drawing.
type Options struct {
	// Width is the room available, in columns. Render picks the layout
	// that fits: it wraps labels tighter and turns a wide left-to-right
	// flowchart top to bottom. 0 means unlimited. If nothing fits, Render
	// returns the narrowest attempt; compare Picture.W with Width.
	Width int
	// ASCII draws with plain ASCII characters only.
	ASCII bool
}

// Role says what a cell is part of, for colouring.
type Role uint8

const (
	RoleNone         Role = iota
	RoleTitle             // the diagram's title
	RoleNode              // a node's border
	RoleNodeText          // the text inside a node
	RoleDecision          // a decision (rhombus) node's border
	RoleStart             // start and end markers, fork bars
	RoleEdge              // a connecting line
	RoleArrow             // an arrowhead
	RoleEdgeLabel         // text on a line
	RoleCluster           // a subgraph's, namespace's or boundary's frame
	RoleClusterTitle      // its title
	RoleNote              // a note's border and text
	RoleActor             // a sequence participant's box
	RoleLifeline          // a sequence participant's lifeline
	RoleBlock             // a sequence loop/alt/opt frame
	RoleBlockTitle        // its label
	RoleBar               // a bar or point of a chart
	RoleMuted             // secondary text (descriptions, values, axes)
	RoleAccent            // highlighted text (active tasks, critical paths)
	RoleWarn              // warnings (critical tasks, low scores)
	RoleGood              // good things (done tasks, high scores)
)

// Cell is one character cell of a Picture. A wide character occupies its
// cell and the next, whose S is empty.
type Cell struct {
	S     string
	Role  Role
	Color string // "#rrggbb" from the diagram's own styling (classDef, style), or ""
	Bold  bool
	wide  bool // continuation of a wide character
}

// Picture is a drawn diagram.
type Picture struct {
	Kind  string // flowchart, sequence, class, state, er, gantt, …
	Title string
	W, H  int
	Cells [][]Cell
}

// UnsupportedError is returned for kinds of diagram that aren't drawn.
type UnsupportedError struct{ Kind string }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s diagrams can't be drawn in a terminal", e.Kind)
}

// ErrEmpty is returned for a diagram with nothing in it.
var ErrEmpty = errors.New("the diagram is empty")

// ErrInvalidUTF8 is returned when Mermaid source contains malformed UTF-8.
// Rust's &str input enforces the same encoding constraint in its type system.
var ErrInvalidUTF8 = errors.New("source is not valid UTF-8")

// renderer draws one kind of diagram. body is the whole source (front
// matter removed), rest what follows the header line. It returns the
// picture and a title found in the source.
type renderer func(body, rest string, opt Options) (*Picture, string, error)

// kinds maps a diagram's header word to its renderer and the name of its
// kind.
var kinds = map[string]struct {
	name string
	draw renderer
}{}

func register(name string, draw renderer, headers ...string) {
	for _, h := range headers {
		kinds[h] = struct {
			name string
			draw renderer
		}{name, draw}
	}
}

func init() {
	graphKind := func(parse func(body, rest string) (*graph, string, error)) renderer {
		return func(body, rest string, opt Options) (*Picture, string, error) {
			var title string
			p, err := fitGraph(func() (*graph, error) {
				g, t, err := parse(body, rest)
				title = t
				return g, err
			}, opt)
			return p, title, err
		}
	}
	register("flowchart", graphKind(func(body, _ string) (*graph, string, error) {
		g, err := parseFlowchart(body)
		return g, "", err
	}), "flowchart", "graph", "flowchart-elk")
	register("state", graphKind(func(_, rest string) (*graph, string, error) {
		g, err := parseState(rest)
		return g, "", err
	}), "stateDiagram", "stateDiagram-v2")
	register("c4", graphKind(func(_, rest string) (*graph, string, error) { return parseC4(rest) }),
		"C4Context", "C4Container", "C4Component", "C4Dynamic", "C4Deployment")
	register("sequence", func(_, rest string, opt Options) (*Picture, string, error) { return renderSequence(rest, opt) },
		"sequenceDiagram")
	register("class", graphKind(parseClass), "classDiagram", "classDiagram-v2")
	register("er", graphKind(parseER), "erDiagram")
	register("gantt", renderGantt, "gantt")
	register("journey", renderJourney, "journey")
	register("timeline", renderTimeline, "timeline")
	register("mindmap", renderMindmap, "mindmap")
	register("gitgraph", renderGitGraph, "gitGraph")
	register("quadrant", renderQuadrant, "quadrantChart")
	register("xychart", renderXYChart, "xychart-beta", "xychart")
	register("kanban", renderKanban, "kanban")
	register("packet", renderPacket, "packet-beta", "packet")
	register("sankey", renderSankey, "sankey-beta", "sankey")
	register("radar", renderRadar, "radar-beta", "radar")
	register("treemap", renderTreemap, "treemap-beta", "treemap")
	register("requirement", graphKind(parseRequirement), "requirementDiagram")
	register("architecture", graphKind(parseArchitecture), "architecture-beta", "architecture")
	register("block", renderBlock, "block-beta", "block")
	register("pie", func(body, _ string, opt Options) (*Picture, string, error) { return renderPie(body, opt) }, "pie")
}

// Render parses and draws a diagram.
func Render(src string, opt Options) (*Picture, error) {
	if !utf8.ValidString(src) {
		return nil, ErrInvalidUTF8
	}
	body, title := Clean(src)
	header, rest := detect(body)
	if header == "" {
		return nil, ErrEmpty
	}
	k, ok := kinds[header]
	if !ok {
		return nil, &UnsupportedError{header}
	}
	p, t, err := k.draw(body, rest, opt)
	if err != nil {
		return nil, err
	}
	if title == "" {
		title = t
	}
	p.Kind, p.Title = k.name, title
	return p, nil
}

// Kinds lists the diagram headers boxmaid draws ("flowchart",
// "sequenceDiagram", …), sorted.
func Kinds() []string {
	var out []string
	for h := range kinds {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// Supported reports whether src is a kind of diagram boxmaid draws.
func Supported(src string) bool {
	_, ok := kinds[Kind(src)]
	return ok
}

var (
	frontmatter = regexp.MustCompile(`(?s)\A\s*---\s*\n(.*?)\n\s*---\s*\n`)
	directive   = regexp.MustCompile(`(?s)%%\{.*?\}%%`)
	fmTitle     = regexp.MustCompile(`(?m)^\s*title\s*:\s*(.+?)\s*$`)
)

// Clean drops front matter, %%{init}%% directives and %% comments, and
// returns the front matter's title if it has one.
func Clean(src string) (body, title string) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\t", "    ")
	if m := frontmatter.FindStringSubmatchIndex(src); m != nil {
		fm := src[m[2]:m[3]]
		if t := fmTitle.FindStringSubmatch(fm); t != nil {
			title = strings.Trim(t[1], `"'`)
		}
		src = src[m[1]:]
	}
	src = directive.ReplaceAllString(src, "")
	var lines []string
	for _, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "%%") {
			continue
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n"), title
}

// detect is the diagram's header (its first word) and the text after the
// header line.
func detect(body string) (kind, rest string) {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		kind = strings.TrimSuffix(strings.Fields(t)[0], ";")
		kind = strings.TrimSuffix(kind, ":")
		return kind, strings.Join(lines[i+1:], "\n")
	}
	return "", ""
}

// Kind is the diagram's header as its source names it ("flowchart",
// "sequenceDiagram", "gantt"…), or "" for an empty one.
func Kind(src string) string {
	body, _ := Clean(src)
	k, _ := detect(body)
	return k
}

// String is the picture as plain text, one line per row, without trailing
// spaces.
func (p *Picture) String() string {
	return strings.Join(p.Lines(), "\n")
}

// Lines is the picture as plain text lines.
func (p *Picture) Lines() []string {
	out := make([]string, len(p.Cells))
	for y, row := range p.Cells {
		var b strings.Builder
		for _, c := range row {
			if !c.wide {
				b.WriteString(c.S)
			}
		}
		out[y] = strings.TrimRight(b.String(), " ")
	}
	return out
}
