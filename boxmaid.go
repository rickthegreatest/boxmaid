// Package boxmaid renders a documented subset of Mermaid as terminal text.
// It provides Unicode/ASCII drawings, best-effort width fitting, semantic
// cell styling, Markdown transformation, and browser fallback links.
//
//	pic, err := boxmaid.Render(src, boxmaid.Options{Width: 80})
//	if err != nil {
//		return err
//	}
//	fmt.Println(pic)
//	fmt.Println(pic.ANSI(boxmaid.NordTheme))
//
// Parsing, layout, and drawing live in an internal package. Public types and
// styling remain here at github.com/rickthegreatest/boxmaid. The core has no
// third-party dependencies.
package boxmaid

import (
	"fmt"
	"strings"

	"github.com/rickthegreatest/boxmaid/internal/renderer"
)

// Options shape the drawing.
type Options struct {
	// Width is the room available, in columns. Render wraps labels and tries
	// alternate directions to fit. Zero means unlimited. If nothing fits,
	// it returns the narrowest attempt; compare Picture.W with Width.
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
	Color string // "#rrggbb" from classDef/style, or ""
	Bold  bool
	wide  bool
}

// Picture is a drawn diagram. Title is metadata, outside the cell grid.
type Picture struct {
	Kind  string
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
var ErrEmpty = renderer.ErrEmpty

// ErrInvalidUTF8 is returned for malformed source encoding.
var ErrInvalidUTF8 = renderer.ErrInvalidUTF8

// Render parses and draws source. Fitting is best-effort: if no attempt
// fits, the narrowest drawing is returned. Unknown statements within a
// supported diagram may be skipped; success is not syntax validation.
func Render(src string, opt Options) (*Picture, error) {
	p, err := renderer.Render(src, renderer.Options{Width: opt.Width, ASCII: opt.ASCII})
	if err != nil {
		switch err {
		case renderer.ErrEmpty:
			return nil, ErrEmpty
		case renderer.ErrInvalidUTF8:
			return nil, ErrInvalidUTF8
		}
		if unsupported, ok := err.(*renderer.UnsupportedError); ok {
			return nil, &UnsupportedError{Kind: unsupported.Kind}
		}
		return nil, err
	}
	return picture(p), nil
}

// picture adapts the internal result to the original public named types.
// Cells share one backing allocation and are read directly from the selected
// canvas, avoiding an intermediate exported grid for each fitting attempt.
func picture(p *renderer.Picture) *Picture {
	out := &Picture{Kind: p.Kind, Title: p.Title, W: p.W, H: p.H}
	if p.W == 0 || p.H == 0 {
		return out
	}
	out.Cells = make([][]Cell, p.H)
	cells := make([]Cell, p.W*p.H)
	for y := range out.Cells {
		out.Cells[y] = cells[y*p.W : (y+1)*p.W : (y+1)*p.W]
		for x := range out.Cells[y] {
			c := p.CellAt(x, y)
			out.Cells[y][x] = Cell{S: c.S, Role: Role(c.Role), Color: c.Color, Bold: c.Bold, wide: c.IsContinuation()}
		}
	}
	return out
}

// String returns plain text without trailing spaces. It excludes Title.
func (p *Picture) String() string { return strings.Join(p.Lines(), "\n") }

// Lines returns the picture as plain text lines, excluding Title.
func (p *Picture) Lines() []string {
	return renderer.PlainLines(p.Cells, func(c Cell) (string, bool) { return c.S, c.wide })
}

// Kinds lists accepted diagram headers in sorted order.
func Kinds() []string { return renderer.Kinds() }

// Supported recognizes a diagram header without validating its statements.
func Supported(src string) bool { return renderer.Supported(src) }

// Kind returns the diagram header, or an empty string for empty source.
func Kind(src string) string { return renderer.Kind(src) }

// Clean removes front matter, directives, and whole-line Mermaid comments.
// The second result is the title found in front matter.
func Clean(src string) (body, title string) { return renderer.Clean(src) }

// ReplaceMermaid replaces renderable fenced Mermaid blocks with fenced text.
// Other code blocks and diagrams that cannot render are preserved.
func ReplaceMermaid(md string, opt Options) string {
	return renderer.ReplaceMermaid(md, renderer.Options{Width: opt.Width, ASCII: opt.ASCII})
}

// LiveURL constructs a mermaid.live URL containing source in its fragment.
// It does not contact a service or open a browser.
func LiveURL(src string) string { return renderer.LiveURL(src) }

// RuneWidth measures a character as zero, one, or two terminal cells.
func RuneWidth(r rune) int { return renderer.RuneWidth(r) }

// StringWidth measures text using Boxmaid's grapheme and cell-width model.
func StringWidth(s string) int { return renderer.StringWidth(s) }
