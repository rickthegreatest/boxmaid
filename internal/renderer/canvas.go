package renderer

// canvas is a grid being drawn on. Lines are recorded as directions per
// cell (up, down, left, right) and turned into box-drawing characters at the
// end, so crossings, junctions and corners join up whatever drew them.
type canvas struct {
	w, h  int
	cells [][]ccell
	ascii bool
}

type ccell struct {
	bits  uint8 // bitUp | bitDown | bitLeft | bitRight
	style lineStyle
	mixed bool // lines of different styles meet here
	dbl   bool // double lines (subroutine sides, double circles)
	round bool // a corner here is rounded
	text  string
	wide  bool // continuation of a wide character to the left
	glyph string
	role  Role
	color string
	bold  bool
}

const (
	bitUp uint8 = 1 << iota
	bitDown
	bitLeft
	bitRight
)

func newCanvas(w, h int, ascii bool) *canvas {
	c := &canvas{w: w, h: h, ascii: ascii}
	c.cells = make([][]ccell, h)
	for y := range c.cells {
		c.cells[y] = make([]ccell, w)
	}
	return c
}

func (c *canvas) at(x, y int) *ccell {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return nil
	}
	return &c.cells[y][x]
}

// rolePriority decides which role a shared cell keeps: borders over lines.
func rolePriority(r Role) int {
	switch r {
	case RoleNode, RoleDecision, RoleActor, RoleNote:
		return 3
	case RoleCluster, RoleBlock:
		return 1
	case RoleEdge, RoleLifeline:
		return 2
	}
	return 0
}

func (c *canvas) addBits(x, y int, bits uint8, st lineStyle, role Role, color string) {
	cell := c.at(x, y)
	if cell == nil {
		return
	}
	if cell.bits != 0 && cell.style != st {
		cell.mixed = true
	}
	cell.bits |= bits
	cell.style = st
	if rolePriority(role) >= rolePriority(cell.role) {
		cell.role = role
		if color != "" || rolePriority(role) > 0 {
			cell.color = color
		}
	}
}

// hline and vline draw a straight line between two cells (inclusive).
func (c *canvas) hline(x1, x2, y int, st lineStyle, role Role, color string) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		var b uint8
		if x > x1 {
			b |= bitLeft
		}
		if x < x2 {
			b |= bitRight
		}
		if x1 == x2 {
			b = bitLeft | bitRight
		}
		c.addBits(x, y, b, st, role, color)
	}
}

func (c *canvas) vline(x, y1, y2 int, st lineStyle, role Role, color string) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		var b uint8
		if y > y1 {
			b |= bitUp
		}
		if y < y2 {
			b |= bitDown
		}
		if y1 == y2 {
			b = bitUp | bitDown
		}
		c.addBits(x, y, b, st, role, color)
	}
}

// box draws a rectangle's border.
func (c *canvas) box(x, y, w, h int, st lineStyle, round, dbl bool, role Role, color string) {
	if w < 2 || h < 2 {
		return
	}
	c.hline(x, x+w-1, y, st, role, color)
	c.hline(x, x+w-1, y+h-1, st, role, color)
	c.vline(x, y, y+h-1, st, role, color)
	c.vline(x+w-1, y, y+h-1, st, role, color)
	for _, p := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		if cell := c.at(p[0], p[1]); cell != nil {
			cell.round = round
		}
	}
	if dbl {
		for yy := y; yy < y+h; yy++ {
			for _, xx := range []int{x, x + w - 1} {
				if cell := c.at(xx, yy); cell != nil {
					cell.dbl = true
				}
			}
		}
		for xx := x; xx < x+w; xx++ {
			for _, yy := range []int{y, y + h - 1} {
				if cell := c.at(xx, yy); cell != nil {
					cell.dbl = true
				}
			}
		}
	}
}

// text writes s from (x, y); wide characters take two cells.
func (c *canvas) text(x, y int, s string, role Role, color string, bold bool) {
	gs, ws := graphemes(s)
	for i, g := range gs {
		cell := c.at(x, y)
		if cell == nil {
			return
		}
		if ws[i] == 2 && c.at(x+1, y) == nil {
			return
		}
		if c.ascii {
			// ASCII mode: every cell gets a plain character of its own.
			a := asciiGrapheme(g, ws[i])
			cell.text, cell.role, cell.color, cell.bold, cell.wide = a[:1], role, color, bold, false
			if ws[i] == 2 {
				n := c.at(x+1, y)
				n.text, n.wide, n.role, n.color, n.bold = a[1:2], false, role, color, bold
			}
			x += ws[i]
			continue
		}
		cell.text, cell.role, cell.color, cell.bold, cell.wide = g, role, color, bold, false
		if ws[i] == 2 {
			n := c.at(x+1, y)
			n.text, n.wide, n.role = "", true, role
		}
		x += ws[i]
	}
}

// glyphAt puts a single symbol (an arrowhead, a corner mark) in a cell.
func (c *canvas) glyphAt(x, y int, g string, role Role, color string) {
	if cell := c.at(x, y); cell != nil {
		cell.glyph, cell.role, cell.color = g, role, color
	}
}

var (
	lightGlyphs = map[uint8]string{
		bitLeft | bitRight: "─", bitLeft: "─", bitRight: "─",
		bitUp | bitDown: "│", bitUp: "│", bitDown: "│",
		bitDown | bitRight: "┌", bitDown | bitLeft: "┐", bitUp | bitRight: "└", bitUp | bitLeft: "┘",
		bitUp | bitDown | bitRight: "├", bitUp | bitDown | bitLeft: "┤",
		bitLeft | bitRight | bitDown: "┬", bitLeft | bitRight | bitUp: "┴",
		bitUp | bitDown | bitLeft | bitRight: "┼",
	}
	heavyGlyphs = map[uint8]string{
		bitLeft | bitRight: "━", bitLeft: "━", bitRight: "━",
		bitUp | bitDown: "┃", bitUp: "┃", bitDown: "┃",
		bitDown | bitRight: "┏", bitDown | bitLeft: "┓", bitUp | bitRight: "┗", bitUp | bitLeft: "┛",
		bitUp | bitDown | bitRight: "┣", bitUp | bitDown | bitLeft: "┫",
		bitLeft | bitRight | bitDown: "┳", bitLeft | bitRight | bitUp: "┻",
		bitUp | bitDown | bitLeft | bitRight: "╋",
	}
	doubleGlyphs = map[uint8]string{
		bitLeft | bitRight: "═", bitLeft: "═", bitRight: "═",
		bitUp | bitDown: "║", bitUp: "║", bitDown: "║",
		bitDown | bitRight: "╔", bitDown | bitLeft: "╗", bitUp | bitRight: "╚", bitUp | bitLeft: "╝",
		bitUp | bitDown | bitRight: "╠", bitUp | bitDown | bitLeft: "╣",
		bitLeft | bitRight | bitDown: "╦", bitLeft | bitRight | bitUp: "╩",
		bitUp | bitDown | bitLeft | bitRight: "╬",
	}
	roundGlyphs = map[uint8]string{
		bitDown | bitRight: "╭", bitDown | bitLeft: "╮", bitUp | bitRight: "╰", bitUp | bitLeft: "╯",
	}
)

func (c *canvas) resolve(cell *ccell) string {
	if cell.wide {
		return ""
	}
	if cell.text != "" {
		return cell.text
	}
	if cell.glyph != "" {
		if c.ascii {
			return asciiGrapheme(cell.glyph, 1)
		}
		return cell.glyph
	}
	if cell.bits == 0 {
		return " "
	}
	b := cell.bits
	if c.ascii {
		switch {
		case b&(bitUp|bitDown) != 0 && b&(bitLeft|bitRight) != 0:
			return "+"
		case b&(bitUp|bitDown) != 0:
			if cell.style == lineDotted && !cell.mixed {
				return ":"
			}
			return "|"
		default:
			switch {
			case cell.style == lineDotted && !cell.mixed:
				return "."
			case cell.style == lineThick && !cell.mixed:
				return "="
			}
			return "-"
		}
	}
	if cell.dbl {
		if g, ok := doubleGlyphs[b]; ok && b&(bitUp|bitDown) != 0 && b&(bitLeft|bitRight) != 0 || ok && !cell.mixed {
			return g
		}
	}
	if !cell.mixed {
		switch cell.style {
		case lineThick:
			return heavyGlyphs[b]
		case lineDotted:
			switch b {
			case bitLeft | bitRight, bitLeft, bitRight:
				return "┄"
			case bitUp | bitDown, bitUp, bitDown:
				return "┆"
			}
		}
	}
	if cell.round {
		if g, ok := roundGlyphs[b]; ok {
			return g
		}
	}
	return lightGlyphs[b]
}

// picture takes ownership of the finished canvas and trims its blank margins.
// Cell resolution is deferred so unsuccessful width-fit attempts do not also
// allocate an exported cell grid. Callers must not draw on the canvas afterward.
func (c *canvas) picture() *Picture {
	minX, minY, maxX, maxY := c.w, c.h, -1, -1
	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			if s := c.resolve(&c.cells[y][x]); s != " " && s != "" || c.cells[y][x].wide {
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	p := &Picture{}
	if maxX < 0 {
		return p
	}
	p.W, p.H = maxX-minX+1, maxY-minY+1
	p.canvas, p.x, p.y = c, minX, minY
	return p
}

// blank reports whether w cells from (x, y) are empty.
func (c *canvas) blank(x, y, w int) bool {
	for i := x; i < x+w; i++ {
		cell := c.at(i, y)
		if cell == nil {
			return false
		}
		if cell.bits != 0 || cell.text != "" || cell.glyph != "" || cell.wide {
			return false
		}
	}
	return true
}
