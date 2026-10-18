package renderer

// Drawing a laid-out level: frames, then edges, then nodes and labels on
// top. Everything is computed in layout axes (o across, r along the flow)
// and converted to screen coordinates here.

// xy converts a layout point to the level's screen coordinates.
func (l *level) xy(o, r int) (x, y int) {
	switch l.dir {
	case "BT":
		return o, l.R - 1 - r
	case "LR":
		return r, o
	case "RL":
		return l.R - 1 - r, o
	}
	return o, r
}

// rect is an item's footprint on screen.
func (l *level) rect(it *item) (x, y, w, h int) {
	x1, y1 := l.xy(it.pos, it.r)
	x2, y2 := l.xy(it.pos+it.osize-1, it.r+it.rsize-1)
	return min(x1, x2), min(y1, y2), abs(x2-x1) + 1, abs(y2-y1) + 1
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (l *level) draw(cv *canvas, x0, y0 int) {
	if l.framed {
		cv.box(x0, y0, l.W, l.H, lineSolid, true, false, RoleCluster, "")
		if t := l.cl.title; t != "" {
			cv.text(x0+2, y0, " "+t+" ", RoleClusterTitle, "", true)
		}
	}
	for _, it := range l.items {
		if it.kind == kCluster {
			x, y, _, _ := l.rect(it)
			it.sub.draw(cv, x0+x, y0+y)
		}
	}
	l.pending = nil
	for _, le := range l.edges {
		if le.e.style != lineInvisible {
			l.drawEdge(cv, le, x0, y0)
		}
	}
	placeEndLabels(cv, l.pending)
	for _, it := range l.items {
		switch it.kind {
		case kNode:
			l.drawNode(cv, it, x0, y0)
		case kLabel:
			x, y, w, h := l.rect(it)
			for i, line := range it.lines {
				lw := width(line)
				ly := y + (h-len(it.lines))/2 + i
				cv.text(x0+x+(w-lw)/2, y0+ly, line, RoleEdgeLabel, "", false)
			}
		}
	}
}

// ends says, for each end of a level edge, whether an arrowhead (or circle,
// cross) is drawn there and which.
func (le *ledge) ends() (aHead, bHead head) {
	toHead, fromHead := le.e.head, le.e.tail
	if le.aIsTo {
		aHead, bHead = toHead, fromHead
	} else {
		aHead, bHead = fromHead, toHead
	}
	if !le.aReal {
		aHead = headNone
	}
	if !le.bReal {
		bHead = headNone
	}
	return
}

func (l *level) drawEdge(cv *canvas, le *ledge, x0, y0 int) {
	chain := le.chain
	if len(chain) < 2 {
		return
	}
	aHead, bHead := le.ends()
	stubA, _, _ := l.stubs()
	first := chain[0]
	var s0 *seg
	for _, s := range first.downs {
		if s.le == le {
			s0 = s
		}
	}
	if s0 == nil {
		return
	}
	type pt struct{ o, r int }
	var pts []pt
	startR := first.r + first.rsize - 1
	if first.kind == kNode && aHead != headNone {
		startR++
	}
	pts = append(pts, pt{first.pos + l.portOff(first, s0), startR})
	s := s0
	for {
		a, b := s.a, s.b
		xa, xb := a.pos+l.portOff(a, s), b.pos+l.portOff(b, s)
		if xa != xb {
			tr := l.gapAt[a.rank] + stubA + l.tracks[s]
			pts = append(pts, pt{xa, tr}, pt{xb, tr})
		}
		if b == le.b {
			endR := b.r
			if b.kind == kNode && bHead != headNone {
				endR--
			}
			pts = append(pts, pt{xb, endR})
			break
		}
		var next *seg
		for _, d := range b.downs {
			if d.le == le {
				next = d
			}
		}
		if next == nil {
			return
		}
		s = next
	}
	// To screen, then draw.
	xs := make([][2]int, len(pts))
	for i, p := range pts {
		x, y := l.xy(p.o, p.r)
		xs[i] = [2]int{x0 + x, y0 + y}
	}
	st := le.e.style
	for i := 0; i+1 < len(xs); i++ {
		p, q := xs[i], xs[i+1]
		switch {
		case p == q:
		case p[0] == q[0]:
			cv.vline(p[0], p[1], q[1], st, RoleEdge, "")
		case p[1] == q[1]:
			cv.hline(p[0], q[0], p[1], st, RoleEdge, "")
		}
	}
	if len(xs) == 1 {
		cv.vline(xs[0][0], xs[0][1], xs[0][1], st, RoleEdge, "")
	}
	if bHead != headNone {
		end := xs[len(xs)-1]
		prev := end
		for i := len(xs) - 2; i >= 0 && prev == end; i-- {
			prev = xs[i]
		}
		cv.glyphAt(end[0], end[1], headGlyph(bHead, end, prev, l, true), RoleArrow, "")
	}
	if le.e.fromLabel != "" || le.e.toLabel != "" {
		fromEnd, toEnd := xs[0], xs[len(xs)-1]
		fromNext, toPrev := xs[min(1, len(xs)-1)], xs[max(0, len(xs)-2)]
		fl, tl := le.e.fromLabel, le.e.toLabel
		if le.aIsTo {
			fl, tl = tl, fl
		}
		l.pending = append(l.pending, endMark{fl, fromEnd, fromNext, aHead == headNone}, endMark{tl, toEnd, toPrev, bHead == headNone})
	}
	if aHead != headNone {
		start := xs[0]
		next := start
		for i := 1; i < len(xs) && next == start; i++ {
			next = xs[i]
		}
		cv.glyphAt(start[0], start[1], headGlyph(aHead, start, next, l, false), RoleArrow, "")
	}
}

// headGlyph points an arrowhead from prev towards at. When the edge has no
// length (prev == at), it points along the flow.
func headGlyph(h head, at, prev [2]int, l *level, forward bool) string {
	switch h {
	case headCircle:
		return "○"
	case headCross:
		return "×"
	case headDiamond:
		return "◆"
	case headDiamondOpen:
		return "◇"
	}
	dx, dy := at[0]-prev[0], at[1]-prev[1]
	if dx == 0 && dy == 0 {
		// Along the flow, reversed for an arrow at the start of an edge.
		fx, fy := l.xy(0, 1)
		ox, oy := l.xy(0, 0)
		dx, dy = fx-ox, fy-oy
		if !forward {
			dx, dy = -dx, -dy
		}
	}
	glyphs := [4]string{"▼", "▲", "►", "◄"}
	if h == headTriangle {
		glyphs = [4]string{"▽", "△", "▷", "◁"}
	}
	switch {
	case dy > 0:
		return glyphs[0]
	case dy < 0:
		return glyphs[1]
	case dx > 0:
		return glyphs[2]
	}
	return glyphs[3]
}

func (l *level) drawNode(cv *canvas, it *item, x0, y0 int) {
	n := it.node
	fx, fy, _, _ := l.rect(it)
	x, y, w, h := x0+fx+it.boxX, y0+fy+it.boxY, it.boxW, it.boxH
	role := RoleNode
	if n.shape == shapeRhombus {
		role = RoleDecision
	}
	st := lineSolid
	if n.dashed {
		st = lineDotted
	}
	col := n.color
	switch n.shape {
	case shapeStart:
		cv.text(x, y, "●", RoleStart, col, false)
		return
	case shapeEnd:
		cv.text(x, y, "◉", RoleStart, col, false)
		return
	case shapeBar:
		if l.lr() {
			for i := range h {
				cv.text(x, y+i, "┃", RoleStart, col, false)
			}
		} else {
			for i := range w {
				cv.text(x+i, y, "━", RoleStart, col, false)
			}
		}
		return
	case shapeText:
	default:
		round := n.shape == shapeRound || n.shape == shapeStadium || n.shape == shapeCircle || n.shape == shapeCylinder
		dbl := n.shape == shapeDoubleCircle || n.shape == shapeSubroutine
		cv.box(x, y, w, h, st, round, dbl, role, col)
		if n.shape == shapeSubroutine {
			for _, p := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
				if c := cv.at(p[0], p[1]); c != nil {
					c.dbl = false
				}
			}
			for xx := x + 1; xx < x+w-1; xx++ {
				for _, yy := range []int{y, y + h - 1} {
					if c := cv.at(xx, yy); c != nil {
						c.dbl = false
					}
				}
			}
		}
		if n.shape == shapeCylinder {
			cv.hline(x, x+w-1, y+1, st, role, col)
		}
		corners := map[shape][4]string{
			shapeRhombus:      {"╱", "╲", "╲", "╱"},
			shapeHexagon:      {"╱", "╲", "╲", "╱"},
			shapeLeanRight:    {"╱", "╱", "╱", "╱"},
			shapeLeanLeft:     {"╲", "╲", "╲", "╲"},
			shapeTrapezoid:    {"╱", "╲", "", ""},
			shapeTrapezoidAlt: {"", "", "╲", "╱"},
		}
		if cs, ok := corners[n.shape]; ok {
			for i, p := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
				if cs[i] != "" {
					cv.glyphAt(p[0], p[1], cs[i], role, col)
				}
			}
		}
	}
	if len(n.header) > 0 || len(n.sections) > 0 {
		l.drawCompartments(cv, it, x, y, w, role, col)
		l.drawLoops(cv, it, x, y, w, h)
		return
	}
	lines := it.lines
	total := len(lines)
	if len(it.detail) > 0 {
		total += 1 + len(it.detail)
	}
	top := y + (h-total)/2
	if n.shape == shapeCylinder {
		top = y + 1 + (h-1-total)/2
	}
	if n.shape == shapeText {
		top = y
	}
	for i, line := range lines {
		cv.text(x+(w-width(line))/2, top+i, line, RoleNodeText, "", true)
	}
	for i, line := range it.detail {
		cv.text(x+(w-width(line))/2, top+len(lines)+1+i, line, RoleMuted, "", false)
	}
	l.drawLoops(cv, it, x, y, w, h)
}

// drawCompartments writes a box made of compartments (a class, an entity, a
// requirement): header and name centred, then each section left-aligned
// below a dividing line.
func (l *level) drawCompartments(cv *canvas, it *item, x, y, w int, role Role, col string) {
	n := it.node
	row := y + 1
	for _, line := range n.header {
		cv.text(x+(w-width(line))/2, row, line, RoleMuted, "", false)
		row++
	}
	for _, line := range it.lines {
		cv.text(x+(w-width(line))/2, row, line, RoleNodeText, "", true)
		row++
	}
	for _, sec := range n.sections {
		cv.hline(x, x+w-1, row, lineSolid, role, col)
		row++
		for _, line := range sec {
			cv.text(x+2, row, line, RoleNone, "", false)
			row++
		}
		if len(sec) == 0 {
			row++
		}
	}
}

// drawLoops draws edges from a node to itself beside it.
func (l *level) drawLoops(cv *canvas, it *item, x, y, w, h int) {
	for i, e := range it.loops {
		if i > 0 || e.style == lineInvisible {
			break // one loop is drawn; more would only overlap
		}
		lbl := ""
		if len(e.label) > 0 {
			lbl = e.label[0]
		}
		if l.lr() {
			cx := x + w/2
			cv.vline(cx+1, y-2, y, e.style, RoleEdge, "")
			cv.hline(cx-1, cx+1, y-2, e.style, RoleEdge, "")
			cv.vline(cx-1, y-2, y-1, e.style, RoleEdge, "")
			cv.glyphAt(cx-1, y-1, "▼", RoleArrow, "")
			if lbl != "" {
				cv.text(cx+3, y-2, lbl, RoleEdgeLabel, "", false)
			}
			continue
		}
		r1, r2 := y+1, y+2
		right := x + w - 1
		cv.hline(right, right+2, r1, e.style, RoleEdge, "")
		cv.vline(right+2, r1, r2, e.style, RoleEdge, "")
		cv.hline(right+1, right+2, r2, e.style, RoleEdge, "")
		cv.glyphAt(right+1, r2, "◄", RoleArrow, "")
		if lbl != "" {
			cv.text(right+4, r1, lbl, RoleEdgeLabel, "", false)
		}
	}
}

// fitGraph lays a graph out in the author's direction, wrapping labels
// tighter and then trying the other direction until it fits the width.
func fitGraph(parse func() (*graph, error), opt Options) (*Picture, error) {
	g, err := parse()
	if err != nil {
		return nil, err
	}
	dirs := []string{g.dir}
	switch g.dir {
	case "LR", "RL":
		dirs = append(dirs, "TB")
	default:
		dirs = append(dirs, "LR")
	}
	var best *Picture
	for _, d := range dirs {
		for _, w := range []int{28, 20, 14} {
			p := layoutGraph(g, d, w, opt.ASCII)
			if opt.Width <= 0 || p.W <= opt.Width {
				return p, nil
			}
			if best == nil || p.W < best.W {
				best = p
			}
		}
	}
	return best, nil
}

// endMark is a multiplicity waiting to be written beside the end of a line.
type endMark struct {
	text      string
	end, next [2]int
	onBorder  bool // the line ends on the box's border (no arrowhead)
}

// placeEndLabels writes multiplicities ("1", "0..*") once all lines are
// drawn: one step out from the box, on whichever side of the line is free.
func placeEndLabels(cv *canvas, marks []endMark) {
	for _, m := range marks {
		if m.text == "" {
			continue
		}
		x, y := m.end[0], m.end[1]
		dx, dy := sign(m.next[0]-x), sign(m.next[1]-y)
		if m.onBorder {
			x, y = x+dx, y+dy
		}
		w := width(m.text)
		var spots [][2]int
		if dx == 0 { // a vertical line: right of it, else left
			spots = [][2]int{{x + 2, y}, {x - 1 - w, y}, {x + 2, y + dy}, {x - 1 - w, y + dy}}
		} else { // horizontal: above, else below
			spots = [][2]int{{x, y - 1}, {x, y + 1}, {x - w + 1, y - 1}}
		}
		for _, p := range spots {
			if cv.blank(p[0]-1, p[1], w+2) {
				cv.text(p[0], p[1], m.text, RoleEdgeLabel, "", false)
				break
			}
		}
	}
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
