package boxmaid

import (
	"fmt"
	"regexp"
	"strings"
)

// Sequence diagrams: participants side by side with lifelines down from
// them, messages as arrows between lifelines, top to bottom in order.

type seqParticipant struct {
	id      string
	label   []string
	actor   bool
	x       int // lifeline column
	w       int // box width
	group   *seqGroup
	created int // row where a created participant appears, or -1
	idx     int
}

type seqGroup struct {
	title string
	first int
	last  int
}

type seqKind uint8

const (
	evMsg seqKind = iota
	evNote
	evBlock
	evElse
	evEnd
	evActivate
	evDeactivate
	evCreate
)

type seqEvent struct {
	kind       seqKind
	from, to   int
	label      []string
	line       lineStyle
	head, tail head
	act, deact bool   // the message activates its target / deactivates its source
	side       string // notes: left, right, over
	block      *seqBlock
	who        int // activate/deactivate/create
}

type seqBlock struct {
	kind   string
	label  string
	parent *seqBlock
	lo, hi int // participants it spans
	depth  int // how many blocks nest inside it
	top    int
	bottom int
	elses  []seqElse
}

type seqElse struct {
	row   int
	label string
	kind  string
}

var (
	seqMsg  = regexp.MustCompile(`^(.+?)\s*(<<-->>|<<->>|-->>|->>|-->|->|--x|-x|--\)|-\))\s*([+-]?)\s*([^:]+?)\s*(?::\s?(.*))?$`)
	seqPart = regexp.MustCompile(`^(participant|actor)\s+(.+?)(?:\s+as\s+(.+))?$`)
	seqNote = regexp.MustCompile(`(?i)^note\s+(left of|right of|over)\s+([^:]+?)\s*:\s*(.*)$`)
)

type seqParser struct {
	parts  []*seqParticipant
	byID   map[string]*seqParticipant
	events []*seqEvent
	blocks []*seqBlock
	stack  []*seqBlock
	group  *seqGroup
	auto   bool
	n      int
	title  string
}

func (p *seqParser) part(id string) *seqParticipant {
	id = strings.TrimSpace(id)
	if q, ok := p.byID[id]; ok {
		return q
	}
	q := &seqParticipant{id: id, label: label(id), created: -1, idx: len(p.parts)}
	if p.group != nil {
		q.group = p.group
		p.group.last = q.idx
	}
	p.parts = append(p.parts, q)
	p.byID[id] = q
	return q
}

// touch widens the open blocks to cover participants a..b.
func (p *seqParser) touch(a, b int) {
	if a > b {
		a, b = b, a
	}
	for _, bl := range p.stack {
		if bl.lo < 0 || a < bl.lo {
			bl.lo = a
		}
		bl.hi = max(bl.hi, b)
	}
}

func parseSequence(src string) (*seqParser, error) {
	p := &seqParser{byID: map[string]*seqParticipant{}}
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
		if line == "" {
			continue
		}
		word, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch strings.ToLower(word) {
		case "title":
			p.title = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
			continue
		case "title:":
			p.title = rest
			continue
		case "autonumber":
			p.auto = true
			continue
		case "link", "links", "acctitle", "accdescr", "acctitle:", "accdescr:", "destroy":
			continue
		case "box":
			p.group = &seqGroup{title: boxTitle(rest), first: len(p.parts), last: len(p.parts) - 1}
			continue
		case "create":
			if m := seqPart.FindStringSubmatch(rest); m != nil {
				q := p.part(m[2])
				q.actor = m[1] == "actor"
				if m[3] != "" {
					q.label = label(m[3])
				}
				p.events = append(p.events, &seqEvent{kind: evCreate, who: q.idx})
			}
			continue
		case "activate", "deactivate":
			q := p.part(rest)
			k := evActivate
			if word == "deactivate" {
				k = evDeactivate
			}
			p.events = append(p.events, &seqEvent{kind: k, who: q.idx})
			continue
		case "loop", "alt", "opt", "par", "critical", "break", "rect":
			lbl := rest
			if word == "rect" {
				lbl = ""
			}
			bl := &seqBlock{kind: word, label: strings.Join(label(lbl), " "), lo: -1, hi: -1}
			if len(p.stack) > 0 {
				bl.parent = p.stack[len(p.stack)-1]
			}
			p.stack = append(p.stack, bl)
			p.blocks = append(p.blocks, bl)
			p.events = append(p.events, &seqEvent{kind: evBlock, block: bl})
			continue
		case "else", "and", "option":
			if len(p.stack) > 0 {
				p.events = append(p.events, &seqEvent{kind: evElse, block: p.stack[len(p.stack)-1], label: []string{word, strings.Join(label(rest), " ")}})
			}
			continue
		case "end":
			if len(p.stack) > 0 {
				p.events = append(p.events, &seqEvent{kind: evEnd, block: p.stack[len(p.stack)-1]})
				p.stack = p.stack[:len(p.stack)-1]
			} else if p.group != nil {
				p.group = nil
			}
			continue
		}
		if m := seqPart.FindStringSubmatch(line); m != nil {
			id, name := m[2], m[3]
			q := p.part(id)
			q.actor = m[1] == "actor"
			if name != "" {
				q.label = label(name)
			}
			continue
		}
		if m := seqNote.FindStringSubmatch(line); m != nil {
			ids := strings.Split(m[2], ",")
			a := p.part(ids[0]).idx
			b := a
			if len(ids) > 1 {
				b = p.part(ids[1]).idx
			}
			if a > b {
				a, b = b, a
			}
			side := strings.ToLower(strings.Fields(m[1])[0])
			p.events = append(p.events, &seqEvent{kind: evNote, from: a, to: b, side: side, label: label(m[3])})
			lo, hi := a, b
			if side == "right" && hi+1 < len(p.parts) {
				hi++
			}
			p.touch(lo, hi)
			continue
		}
		if m := seqMsg.FindStringSubmatch(line); m != nil {
			from, to := p.part(m[1]), p.part(m[4])
			e := &seqEvent{kind: evMsg, from: from.idx, to: to.idx, label: label(m[5])}
			if len(e.label) == 1 && e.label[0] == "" {
				e.label = nil
			}
			op := m[2]
			if strings.Contains(strings.Trim(op, "<"), "--") {
				e.line = lineDotted
			}
			switch {
			case strings.HasSuffix(op, ">>"):
				e.head = headArrow
			case strings.HasSuffix(op, "x"):
				e.head = headCross
			case strings.HasSuffix(op, ")"):
				e.head = headArrow
			}
			if strings.HasPrefix(op, "<<") {
				e.tail = headArrow
			}
			e.act, e.deact = m[3] == "+", m[3] == "-"
			if p.auto {
				p.n++
				e.label = append([]string{fmt.Sprintf("%s %s", circled(p.n), first(e.label))}, rest1(e.label)...)
			}
			p.events = append(p.events, e)
			hi := max(from.idx, to.idx)
			if from == to && hi+1 < len(p.parts) {
				hi++ // a message to itself loops out to the right
			}
			p.touch(min(from.idx, to.idx), hi)
			continue
		}
	}
	if len(p.parts) == 0 {
		return nil, ErrEmpty
	}
	return p, nil
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func rest1(s []string) []string {
	if len(s) < 2 {
		return nil
	}
	return s[1:]
}

func circled(n int) string {
	if n >= 1 && n <= 20 {
		return string(rune('①' + n - 1))
	}
	return fmt.Sprintf("(%d)", n)
}

// boxTitle drops a box's colour ("box Aqua Payments", "box rgb(…) Payments").
func boxTitle(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "rgb") || strings.HasPrefix(s, "#") {
		if i := strings.IndexAny(s, ") "); i >= 0 {
			s = strings.TrimSpace(s[i+1:])
		}
	} else if f := strings.Fields(s); len(f) > 1 && isColorName(f[0]) {
		s = strings.Join(f[1:], " ")
	}
	return strings.Join(label(s), " ")
}

func isColorName(s string) bool {
	switch strings.ToLower(s) {
	case "aqua", "red", "green", "blue", "yellow", "orange", "purple", "pink", "grey", "gray", "white", "black",
		"lightblue", "lightgreen", "lightyellow", "lightgrey", "lightgray", "transparent", "teal", "olive", "silver", "beige":
		return true
	}
	return false
}

// renderSequence parses and draws a sequence diagram, wrapping message
// labels tighter until it fits.
func renderSequence(src string, opt Options) (*Picture, string, error) {
	p, err := parseSequence(src)
	if err != nil {
		return nil, "", err
	}
	var best *Picture
	for _, wrapAt := range []int{40, 28, 20, 14} {
		pic := p.draw(wrapAt, opt.ASCII)
		if opt.Width <= 0 || pic.W <= opt.Width {
			return pic, p.title, nil
		}
		if best == nil || pic.W < best.W {
			best = pic
		}
	}
	return best, p.title, nil
}

func (p *seqParser) draw(wrapAt int, ascii bool) *Picture {
	parts := p.parts
	n := len(parts)
	for _, q := range parts {
		q.w = maxWidth(wrap(q.label, 20)) + 4
	}
	labels := map[*seqEvent][]string{}
	for _, e := range p.events {
		if e.kind == evMsg || e.kind == evNote {
			labels[e] = wrap(e.label, wrapAt)
		}
	}
	// Gaps between neighbouring lifelines.
	gap := make([]int, max(0, n-1))
	for i := range gap {
		gap[i] = (parts[i].w+parts[i+1].w)/2 + 3
		if parts[i].group != parts[i+1].group {
			gap[i] += 2
		}
	}
	right := parts[n-1].w/2 + 2 // room right of the last lifeline
	left := parts[0].w/2 + 2
	need := func(a, b, w int) { // lifelines a..b (a<b) must be w apart
		sum := 0
		for i := a; i < b; i++ {
			sum += gap[i]
		}
		if sum < w {
			gap[b-1] += w - sum
		}
	}
	for _, e := range p.events {
		lw := maxWidth(labels[e])
		switch e.kind {
		case evMsg:
			a, b := min(e.from, e.to), max(e.from, e.to)
			if a == b {
				if a < n-1 {
					need(a, a+1, lw+8)
				} else {
					right = max(right, lw+8)
				}
				continue
			}
			need(a, b, lw+6)
		case evNote:
			nw := lw + 4
			switch e.side {
			case "right":
				if e.from < n-1 {
					need(e.from, e.from+1, nw+3)
				} else {
					right = max(right, nw+3)
				}
			case "left":
				if e.from > 0 {
					need(e.from-1, e.from, nw+3)
				} else {
					left = max(left, nw+3)
				}
			default:
				if e.from == e.to {
					if e.from == 0 {
						left = max(left, nw/2+1)
					}
					if e.from == n-1 {
						right = max(right, nw/2+1)
					}
				} else {
					need(e.from, e.to, nw-4)
				}
			}
		}
	}
	for _, bl := range p.blocks {
		if bl.lo < 0 {
			bl.lo, bl.hi = 0, n-1
		}
		d := 0
		for c := bl; c.parent != nil; c = c.parent {
			c.parent.depth = max(c.parent.depth, d+1)
			d++
		}
		lw := width(bl.kind) + width(bl.label) + 8
		if bl.lo < bl.hi {
			need(bl.lo, bl.hi, lw-6)
		}
	}
	maxDepth := 0
	for _, bl := range p.blocks {
		maxDepth = max(maxDepth, bl.depth+1)
	}
	left += 3 + 2*maxDepth
	right += 3 + 2*maxDepth
	x := left
	for i, q := range parts {
		q.x = x
		if i < n-1 {
			x += gap[i]
		}
	}
	W := x + right
	// Rows.
	boxH := 3
	for _, q := range parts {
		boxH = max(boxH, len(wrap(q.label, 20))+2)
	}
	top := 0
	hasGroups := false
	for _, q := range parts {
		if q.group != nil {
			hasGroups = true
		}
	}
	if hasGroups {
		top = 2
	}
	y := top + boxH + 1
	rows := map[*seqEvent]int{}
	activeFrom := make([][]int, n)
	type span struct{ who, from, to int }
	var spans []span
	activate := func(who, row int) { activeFrom[who] = append(activeFrom[who], row) }
	deactivate := func(who, row int) {
		if k := len(activeFrom[who]); k > 0 {
			spans = append(spans, span{who, activeFrom[who][k-1], row})
			activeFrom[who] = activeFrom[who][:k-1]
		}
	}
	for _, e := range p.events {
		switch e.kind {
		case evCreate:
			parts[e.who].created = y
			y += boxH
		case evMsg:
			lines := labels[e]
			if e.from == e.to {
				rows[e] = y
				y += max(2, len(lines)+1) + 1
				continue
			}
			y += len(lines)
			rows[e] = y
			if e.act {
				activate(e.to, y)
			}
			if e.deact {
				deactivate(e.from, y)
			}
			y += 2
		case evNote:
			rows[e] = y
			y += len(labels[e]) + 3
		case evBlock:
			e.block.top = y
			e.block.elses = nil // from an earlier, wider attempt
			y += 2
		case evElse:
			e.block.elses = append(e.block.elses, seqElse{row: y, kind: e.label[0], label: e.label[1]})
			y += 2
		case evEnd:
			e.block.bottom = y
			y += 2
		case evActivate:
			activate(e.who, y)
		case evDeactivate:
			deactivate(e.who, y)
		}
	}
	for who := range activeFrom {
		for len(activeFrom[who]) > 0 {
			deactivate(who, y)
		}
	}
	bottomY := y
	H := bottomY + boxH + 1
	if hasGroups {
		H++
	}
	cv := newCanvas(W+2, H+2, ascii)
	// Participant groups.
	for _, q := range parts {
		if g := q.group; g != nil && q.idx == g.first {
			l, r := parts[g.first], parts[max(g.first, g.last)]
			x1, x2 := l.x-l.w/2-2, r.x+r.w/2+2
			cv.box(x1, 0, x2-x1+1, H, lineDotted, true, false, RoleCluster, "")
			if g.title != "" {
				cv.text(x1+2, 0, " "+g.title+" ", RoleClusterTitle, "", true)
			}
		}
	}
	// Blocks.
	for _, bl := range p.blocks {
		l, r := parts[bl.lo], parts[bl.hi]
		m := 3 + 2*bl.depth
		x1, x2 := l.x-m, r.x+m
		if bl.hi == n-1 {
			x2 = max(x2, W-2-2*(maxDepth-1-bl.depth))
		}
		if bl.top == 0 && bl.bottom == 0 {
			continue
		}
		if bl.bottom <= bl.top {
			bl.bottom = bottomY - 1
		}
		cv.box(x1, bl.top, x2-x1+1, bl.bottom-bl.top+1, lineSolid, false, false, RoleBlock, "")
		if bl.kind != "rect" {
			cv.text(x1+1, bl.top, " "+bl.kind+" ", RoleBlockTitle, "", true)
			if bl.label != "" {
				cv.text(x1+width(bl.kind)+3, bl.top, "["+bl.label+"] ", RoleBlockTitle, "", false)
			}
		}
		for _, el := range bl.elses {
			cv.hline(x1, x2, el.row, lineDotted, RoleBlock, "")
			cv.addBits(x1, el.row, bitUp|bitDown, lineSolid, RoleBlock, "")
			cv.addBits(x2, el.row, bitUp|bitDown, lineSolid, RoleBlock, "")
			t := " " + el.kind + " "
			if el.label != "" {
				t += "[" + el.label + "] "
			}
			cv.text(x1+1, el.row, t, RoleBlockTitle, "", false)
		}
	}
	// Lifelines, thick where active.
	for _, q := range parts {
		from := top + boxH
		if q.created >= 0 {
			from = q.created + boxH
		}
		cv.vline(q.x, from, bottomY, lineDotted, RoleLifeline, "")
	}
	for _, s := range spans {
		for r := s.from; r <= s.to; r++ {
			if c := cv.at(parts[s.who].x, r); c != nil {
				c.style, c.mixed = lineThick, false
			}
		}
	}
	// Messages.
	for _, e := range p.events {
		switch e.kind {
		case evMsg:
			lines := labels[e]
			r := rows[e]
			from, to := parts[e.from], parts[e.to]
			if e.from == e.to {
				x := from.x
				cv.hline(x, x+3, r, e.line, RoleEdge, "")
				cv.vline(x+3, r, r+1, e.line, RoleEdge, "")
				cv.hline(x+1, x+3, r+1, e.line, RoleEdge, "")
				if e.head != headNone {
					cv.glyphAt(x+1, r+1, headGlyphFor(e.head, "◄"), RoleArrow, "")
				}
				for i, line := range lines {
					cv.text(x+5, r+i, line, RoleEdgeLabel, "", false)
				}
				continue
			}
			dir := 1
			if to.x < from.x {
				dir = -1
			}
			cv.hline(from.x, to.x-dir, r, e.line, RoleEdge, "")
			if e.head != headNone {
				g := "►"
				if dir < 0 {
					g = "◄"
				}
				cv.glyphAt(to.x-dir, r, headGlyphFor(e.head, g), RoleArrow, "")
			}
			if e.tail != headNone {
				g := "◄"
				if dir < 0 {
					g = "►"
				}
				cv.glyphAt(from.x+dir, r, g, RoleArrow, "")
			}
			mid := (from.x + to.x) / 2
			for i, line := range lines {
				cv.text(mid-width(line)/2, r-len(lines)+i, line, RoleEdgeLabel, "", false)
			}
		case evNote:
			lines := labels[e]
			nw := maxWidth(lines) + 4
			a, b := parts[e.from], parts[e.to]
			var x1 int
			switch e.side {
			case "right":
				x1 = a.x + 2
			case "left":
				x1 = a.x - 2 - nw + 1
			default:
				if e.from == e.to {
					x1 = a.x - nw/2
				} else {
					x1 = a.x - 2
					nw = max(nw, b.x-a.x+5)
				}
			}
			r := rows[e]
			// A note covers the lifelines behind it.
			for yy := r; yy < r+len(lines)+2; yy++ {
				for xx := x1; xx < x1+nw; xx++ {
					if c := cv.at(xx, yy); c != nil {
						*c = ccell{}
					}
				}
			}
			cv.box(x1, r, nw, len(lines)+2, lineSolid, false, false, RoleNote, "")
			for i, line := range lines {
				cv.text(x1+2, r+1+i, line, RoleNote, "", false)
			}
		}
	}
	// Participant boxes, top and bottom.
	for _, q := range parts {
		lines := wrap(q.label, 20)
		ys := []int{top, bottomY + 1}
		if q.created >= 0 {
			ys[0] = q.created
		}
		for _, by := range ys {
			bx := q.x - q.w/2
			cv.box(bx, by, q.w, boxH, lineSolid, q.actor, false, RoleActor, "")
			for i, line := range lines {
				cv.text(q.x-width(line)/2, by+1+(boxH-2-len(lines))/2+i, line, RoleNodeText, "", true)
			}
		}
	}
	return cv.picture()
}

func headGlyphFor(h head, arrow string) string {
	switch h {
	case headCross:
		return "×"
	case headCircle:
		return "○"
	}
	return arrow
}
