package boxmaid

import (
	"regexp"
	"strconv"
	"strings"
)

// Block diagrams put blocks on the grid their source describes: "columns
// 3", blocks that span columns ("a:2"), empty space, and nested blocks
// with columns of their own. Edges run through the gaps between rows, each
// on its own track.

type blockItem struct {
	id       string
	text     []string
	span     int
	space    bool
	shape    shape
	children *blockGrid // a nested block
	// placed
	x, y, w, h int
}

type blockGrid struct {
	columns int
	items   []*blockItem
	title   string
}

var (
	blockTok  = regexp.MustCompile(`^([\w-]+)(?:(\(\(|\(\[|\[\(|\[\[|\{\{|\(|\[|\{|>)"?(.*?)"?(\)\)|\]\)|\)\]|\]\]|\}\}|\)|\]|\}))?(?::(\d+))?$`)
	blockEdge = regexp.MustCompile(`^([\w-]+)\s*(?:--\s*"([^"]*)"\s*)?(-->|---|==>|-\.->)\s*([\w-]+)$`)
)

func parseBlockTokens(line string) []string {
	// Split on spaces outside brackets and quotes.
	var out []string
	depth, quoted, start := 0, false, 0
	for i, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case strings.ContainsRune("[({", r):
			depth++
		case strings.ContainsRune("])}", r):
			depth = max(0, depth-1)
		case r == ' ' && depth == 0:
			if i > start {
				out = append(out, line[start:i])
			}
			start = i + 1
		}
	}
	if start < len(line) {
		out = append(out, line[start:])
	}
	return out
}

func renderBlock(_, rest string, opt Options) (*Picture, string, error) {
	root := &blockGrid{}
	stack := []*blockGrid{root}
	type link struct {
		from, to, label string
		style           lineStyle
	}
	var links []link
	all := map[string]*blockItem{}
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "style ") || strings.HasPrefix(line, "classDef ") || strings.HasPrefix(line, "class ") {
			continue
		}
		cur := stack[len(stack)-1]
		if strings.HasPrefix(line, "columns ") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "columns ")))
			if err == nil {
				cur.columns = n
			}
			continue
		}
		if line == "end" {
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if m := blockEdge.FindStringSubmatch(line); m != nil {
			st := lineSolid
			switch m[3] {
			case "==>":
				st = lineThick
			case "-.->":
				st = lineDotted
			}
			links = append(links, link{m[1], m[4], m[2], st})
			continue
		}
		if strings.HasPrefix(line, "block") {
			// block, block:id, block:id:2, block:id["Title"]:2
			spec := strings.TrimPrefix(line, "block")
			it := &blockItem{span: 1, children: &blockGrid{}}
			parts := strings.Split(strings.TrimPrefix(spec, ":"), ":")
			if len(parts) > 0 && parts[0] != "" {
				id := parts[0]
				if i := strings.IndexAny(id, "[("); i > 0 {
					it.children.title = strings.Trim(id[i:], `[]()"`)
					id = id[:i]
				}
				it.id = id
				all[id] = it
			}
			if len(parts) > 1 {
				if n, err := strconv.Atoi(parts[1]); err == nil {
					it.span = n
				}
			}
			cur.items = append(cur.items, it)
			stack = append(stack, it.children)
			continue
		}
		for _, tok := range parseBlockTokens(line) {
			if tok == "space" || strings.HasPrefix(tok, "space:") {
				n := 1
				if _, v, ok := strings.Cut(tok, ":"); ok {
					n, _ = strconv.Atoi(v)
				}
				cur.items = append(cur.items, &blockItem{space: true, span: max(1, n)})
				continue
			}
			m := blockTok.FindStringSubmatch(tok)
			if m == nil {
				continue
			}
			it := &blockItem{id: m[1], text: []string{m[1]}, span: 1}
			if m[3] != "" {
				it.text = label(m[3])
			}
			it.shape = map[string]shape{"(": shapeRound, "((": shapeCircle, "([": shapeStadium, "[(": shapeCylinder,
				"[[": shapeSubroutine, "{": shapeRhombus, "{{": shapeHexagon, ">": shapeAsym}[m[2]]
			if m[5] != "" {
				it.span, _ = strconv.Atoi(m[5])
				it.span = max(1, it.span)
			}
			cur.items = append(cur.items, it)
			all[it.id] = it
		}
	}
	if len(root.items) == 0 {
		return nil, "", ErrEmpty
	}
	// Measure and place, innermost grids first.
	cellW := 0
	var measure func(g *blockGrid)
	measure = func(g *blockGrid) {
		for _, it := range g.items {
			if it.children != nil {
				measure(it.children)
				continue
			}
			if !it.space {
				cellW = max(cellW, maxWidth(it.text)+4)
			}
		}
	}
	measure(root)
	cellW = max(cellW, 7)
	const gapX = 3
	gapY := 3
	var place func(g *blockGrid, x0, y0 int) (int, int)
	place = func(g *blockGrid, x0, y0 int) (int, int) {
		cols := g.columns
		if cols <= 0 {
			cols = 0
			for _, it := range g.items {
				cols += it.span
			}
		}
		col, rowY, rowH, maxW := 0, y0, 0, 0
		for _, it := range g.items {
			span := min(it.span, cols)
			if col+span > cols {
				col = 0
				rowY += rowH + gapY
				rowH = 0
			}
			x := x0 + col*(cellW+gapX)
			w := span*cellW + (span-1)*gapX
			h := 3
			if it.children != nil {
				// A blank row under the frame, so an arrow into a block
				// inside has room.
				cw, ch := place(it.children, x+2, rowY+2)
				w = max(w, cw+4)
				h = ch + 3
			} else if !it.space {
				h = len(it.text) + 2
			}
			it.x, it.y, it.w, it.h = x, rowY, w, h
			rowH = max(rowH, h)
			maxW = max(maxW, x+w-x0)
			col += span
		}
		return maxW, rowY + rowH - y0
	}
	W, H := place(root, 0, 0)
	// Each edge leaving a row downwards (or upwards) needs a track in that
	// gap; widen the gaps when there are more than fit, and place again.
	need := map[int]int{}
	for _, l := range links {
		if a, b := all[l.from], all[l.to]; a != nil && b != nil && !(a.y == b.y && a.h == b.h) {
			need[a.y]++
		}
	}
	most := 0
	for _, n := range need {
		most = max(most, n)
	}
	if most+2 > gapY {
		gapY = most + 2
		W, H = place(root, 0, 0)
	}
	cv := newCanvas(W+2, H+3+gapY, opt.ASCII)
	var draw func(g *blockGrid)
	draw = func(g *blockGrid) {
		for _, it := range g.items {
			switch {
			case it.space:
			case it.children != nil:
				cv.box(it.x, it.y, it.w, it.h, lineSolid, true, false, RoleCluster, "")
				if it.children.title != "" {
					cv.text(it.x+2, it.y, " "+it.children.title+" ", RoleClusterTitle, "", true)
				}
				draw(it.children)
			default:
				round := it.shape == shapeRound || it.shape == shapeStadium || it.shape == shapeCircle || it.shape == shapeCylinder
				role := RoleNode
				if it.shape == shapeRhombus {
					role = RoleDecision
				}
				cv.box(it.x, it.y, it.w, it.h, lineSolid, round, it.shape == shapeSubroutine, role, "")
				for i, l := range it.text {
					cv.text(it.x+(it.w-width(l))/2, it.y+1+i, l, RoleNodeText, "", true)
				}
			}
		}
	}
	draw(root)
	// Edges: straight across when side by side, else down into the gap
	// below the source, along its own track, and into the target.
	trackOf := map[int]int{}
	for _, l := range links {
		a, b := all[l.from], all[l.to]
		if a == nil || b == nil {
			continue
		}
		if a.y == b.y && a.h == b.h { // same row: side to side
			y := a.y + a.h/2
			x1, x2 := a.x+a.w, b.x-1
			if b.x < a.x {
				x1, x2 = a.x-1, b.x+b.w
			}
			cv.hline(min(x1, x2), max(x1, x2), y, l.style, RoleEdge, "")
			g := "►"
			if b.x < a.x {
				g = "◄"
			}
			cv.glyphAt(x2, y, g, RoleArrow, "")
			if l.label != "" {
				cv.text((x1+x2)/2-width(l.label)/2, y-1, l.label, RoleEdgeLabel, "", false)
			}
			continue
		}
		down := b.y > a.y
		sx := a.x + a.w/2
		tx := b.x + b.w/2
		var sy, ty, trackY int
		if down {
			sy, ty = a.y+a.h, b.y-1
			trackY = a.y + a.h + 1 + trackOf[a.y]
		} else {
			sy, ty = a.y-1, b.y+b.h
			trackY = a.y - 2 - trackOf[a.y]
		}
		trackOf[a.y]++
		cv.vline(sx, sy, trackY, l.style, RoleEdge, "")
		cv.hline(sx, tx, trackY, l.style, RoleEdge, "")
		cv.vline(tx, trackY, ty, l.style, RoleEdge, "")
		g := "▼"
		if !down {
			g = "▲"
		}
		cv.glyphAt(tx, ty, g, RoleArrow, "")
		if l.label != "" { // beside the line where it leaves the block
			if cv.blank(sx+1, sy, width(l.label)+2) {
				cv.text(sx+2, sy, l.label, RoleEdgeLabel, "", false)
			} else if cv.blank(tx+1, ty, width(l.label)+2) {
				cv.text(tx+2, ty, l.label, RoleEdgeLabel, "", false)
			}
		}
	}
	return cv.picture(), "", nil
}
