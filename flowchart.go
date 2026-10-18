package boxmaid

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	flowHeader = regexp.MustCompile(`^\s*(?:flowchart|graph)(?:-elk)?\b\s*(TB|TD|BT|LR|RL|<|>|\^|v)?\s*;?\s*(.*)$`)
	// An edge without a label in the middle: -->, ---, -.->, ==>, <-->, o--o, ~~~ …
	edgePlain = regexp.MustCompile(`^(<|o|x)?(?:(-{2,})|(={2,})|(-\.+-)|(~{3,}))(>|o|x)?`)
	// The opening half of an edge with its label in the middle: -- text -->.
	edgeOpen = regexp.MustCompile(`^(<|o|x)?(--|==|-\.)\s`)
	closers  = map[string]*regexp.Regexp{
		"--": regexp.MustCompile(`(-{2,})([>ox])|(-{3,})`),
		"==": regexp.MustCompile(`(={2,})([>ox])|(={3,})`),
		"-.": regexp.MustCompile(`(\.+-)([>ox])?`),
	}
	pipeLabel = regexp.MustCompile(`^\s*\|([^|]*)\|`)
	edgeID    = regexp.MustCompile(`^[A-Za-z0-9_]+@`)
	metaStmt  = regexp.MustCompile(`^([A-Za-z0-9_]+)@\{(.*)\}$`)
	styleKV   = regexp.MustCompile(`(stroke|fill|color)\s*:\s*(#[0-9a-fA-F]{3,8}|[a-zA-Z]+)`)
)

func dirOf(s string) string {
	switch s {
	case "LR", ">":
		return "LR"
	case "RL", "<":
		return "RL"
	case "BT", "^":
		return "BT"
	}
	return "TB"
}

// parseFlowchart reads a flowchart (or graph) definition.
func parseFlowchart(src string) (*graph, error) {
	var g *graph
	var stack []*cluster
	classColor := map[string]string{}
	styled := map[string]string{} // node id → colour from a style statement
	for _, raw := range strings.Split(src, "\n") {
		for _, st := range splitStatements(raw) {
			st = strings.TrimSpace(st)
			if st == "" {
				continue
			}
			if g == nil {
				m := flowHeader.FindStringSubmatch(st)
				if m == nil {
					return nil, fmt.Errorf("not a flowchart: %q", st)
				}
				g = newGraph(dirOf(m[1]))
				stack = []*cluster{g.root}
				if strings.TrimSpace(m[2]) == "" {
					continue
				}
				st = m[2]
			}
			cur := stack[len(stack)-1]
			word, rest, _ := strings.Cut(st, " ")
			rest = strings.TrimSpace(rest)
			switch word {
			case "subgraph":
				id, title := subgraphHead(rest)
				stack = append(stack, g.addCluster(id, title, cur))
				continue
			case "end":
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
				continue
			case "direction":
				if cur == g.root {
					g.dir = dirOf(strings.TrimSpace(rest))
				} else {
					cur.dir = dirOf(strings.TrimSpace(rest))
				}
				continue
			case "classDef":
				names, props, _ := strings.Cut(rest, " ")
				if c := colorOf(props); c != "" {
					for _, n := range strings.Split(names, ",") {
						classColor[strings.TrimSpace(n)] = c
					}
				}
				continue
			case "class":
				f := strings.Fields(rest)
				if len(f) >= 2 {
					for _, id := range strings.Split(f[0], ",") {
						if n, ok := g.nodes[strings.TrimSpace(id)]; ok {
							n.classes = append(n.classes, f[1])
						} else {
							// Classes may be assigned before the node appears.
							n := g.node(strings.TrimSpace(id), cur)
							n.classes = append(n.classes, f[1])
						}
					}
				}
				continue
			case "style":
				id, props, _ := strings.Cut(rest, " ")
				if c := colorOf(props); c != "" {
					styled[id] = c
				}
				continue
			case "linkStyle", "click", "accTitle", "accDescr", "accTitle:", "accDescr:", "callback", "interpolate":
				continue
			}
			if edgeMeta(g, st) {
				continue
			}
			parseChain(g, cur, st)
		}
	}
	if g == nil {
		return nil, ErrEmpty
	}
	for _, n := range g.nodes {
		for _, c := range n.classes {
			if col, ok := classColor[c]; ok {
				n.color = col
			}
		}
		if col, ok := styled[n.id]; ok {
			n.color = col
		}
	}
	g.finish()
	if len(g.nodes) == 0 && len(g.clusters) == 0 {
		return nil, ErrEmpty
	}
	return g, nil
}

// splitStatements splits a line at semicolons outside quotes and brackets.
func splitStatements(line string) []string {
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
		case r == ';' && depth == 0:
			out = append(out, line[start:i])
			start = i + 1
		}
	}
	return append(out, line[start:])
}

// subgraphHead reads "id [title]", "id[title]", "\"title\"" or "some title".
func subgraphHead(s string) (id, title string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "[(\"{"); i > 0 && s[i] != '"' {
		id = strings.TrimSpace(s[:i])
		title = strings.Join(label(strings.Trim(s[i:], "[](){}")), " ")
		return id, title
	}
	title = strings.Join(label(s), " ")
	return title, title
}

// colorOf picks the colour a node is drawn in from Mermaid style props:
// the stroke if there is one, else the fill.
func colorOf(props string) string {
	var stroke, fill string
	for _, m := range styleKV.FindAllStringSubmatch(props, -1) {
		c := normColor(m[2])
		switch m[1] {
		case "stroke":
			stroke = c
		case "fill":
			fill = c
		}
	}
	// Strokes are often plain black or white outlines; the fill says more.
	if stroke != "" && !greyish(stroke) {
		return stroke
	}
	if fill != "" && !greyish(fill) {
		return fill
	}
	return ""
}

var namedColors = map[string]string{
	"red": "#e5484d", "green": "#30a46c", "blue": "#3e63dd", "orange": "#f76b15", "yellow": "#ffc53d",
	"purple": "#8e4ec6", "pink": "#d6409f", "cyan": "#00a2c7", "teal": "#12a594", "lightblue": "#7cc4fa",
	"lightgreen": "#7ee0a1", "gold": "#ffc53d", "violet": "#8e4ec6", "magenta": "#d6409f",
}

func normColor(c string) string {
	c = strings.ToLower(c)
	if n, ok := namedColors[c]; ok {
		return n
	}
	if !strings.HasPrefix(c, "#") {
		return ""
	}
	h := c[1:]
	switch len(h) {
	case 3, 4:
		return "#" + string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	case 6, 8:
		return "#" + h[:6]
	}
	return ""
}

// greyish colours (black, white, greys) carry no meaning as a node colour.
func greyish(c string) bool {
	if len(c) != 7 {
		return true
	}
	var r, g, b int
	fmt.Sscanf(c[1:], "%02x%02x%02x", &r, &g, &b)
	return max(r, g, b)-min(r, g, b) < 24
}

// parseChain reads "A[x] --> B & C -.-> D".
func parseChain(g *graph, c *cluster, s string) {
	p := &chainParser{g: g, c: c, s: s}
	left := p.nodeGroup()
	if len(left) == 0 {
		return
	}
	for {
		p.space()
		if p.done() {
			return
		}
		e, ok := p.edgeOp()
		if !ok {
			return
		}
		right := p.nodeGroup()
		if len(right) == 0 {
			return
		}
		for _, l := range left {
			for _, r := range right {
				x := *e
				x.from, x.to = l, r
				g.edges = append(g.edges, &x)
			}
		}
		left = right
	}
}

type chainParser struct {
	g *graph
	c *cluster
	s string
	i int
}

func (p *chainParser) done() bool { return p.i >= len(p.s) }
func (p *chainParser) rest() string {
	return p.s[p.i:]
}
func (p *chainParser) space() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *chainParser) nodeGroup() []string {
	var ids []string
	for {
		id := p.nodeRef()
		if id == "" {
			return ids
		}
		ids = append(ids, id)
		p.space()
		if !strings.HasPrefix(p.rest(), "&") {
			return ids
		}
		p.i++
	}
}

// atEdge reports whether an edge starts at the cursor.
func (p *chainParser) atEdge() bool {
	r := p.rest()
	if edgeOpen.MatchString(r) {
		return true
	}
	m := edgePlain.FindStringSubmatch(r)
	if m == nil {
		return false
	}
	// o-- and x-- only start an edge after a space ("A o--o B").
	if m[1] == "o" || m[1] == "x" {
		return p.i == 0 || p.s[p.i-1] == ' '
	}
	// A bare "--" or "==" is only half an edge.
	return !(m[6] == "" && (m[2] == "--" || m[3] == "=="))
}

func (p *chainParser) nodeRef() string {
	p.space()
	start := p.i
	for p.i < len(p.s) {
		r := rune(p.s[p.i])
		if r == ' ' || r == '\t' || strings.ContainsRune("[({>&;|\"", r) || strings.HasPrefix(p.rest(), ":::") || strings.HasPrefix(p.rest(), "@{") {
			break
		}
		if p.i > start && p.atEdge() {
			break
		}
		if r >= 0x80 {
			// Multi-byte: part of an identifier.
			_, size := utf8.DecodeRuneInString(p.s[p.i:])
			p.i += size
			continue
		}
		p.i++
	}
	id := p.s[start:p.i]
	if id == "" {
		return ""
	}
	n := p.g.node(id, p.c)
	if strings.HasPrefix(p.rest(), "@{") {
		end := strings.Index(p.rest(), "}")
		if end > 0 {
			p.shapeData(n, p.rest()[2:end])
			p.i += end + 1
		}
	}
	p.shape(n)
	if strings.HasPrefix(p.rest(), ":::") {
		p.i += 3
		cs := p.i
		for p.i < len(p.s) {
			r, size := utf8.DecodeRuneInString(p.s[p.i:])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
				break
			}
			p.i += size
		}
		n.classes = append(n.classes, p.s[cs:p.i])
	}
	return id
}

var shapeOpeners = []struct {
	open   string
	closes []string
	shapes []shape
}{
	{"(((", []string{")))"}, []shape{shapeDoubleCircle}},
	{"((", []string{"))"}, []shape{shapeCircle}},
	{"([", []string{"])"}, []shape{shapeStadium}},
	{"[[", []string{"]]"}, []shape{shapeSubroutine}},
	{"[(", []string{")]"}, []shape{shapeCylinder}},
	{"{{", []string{"}}"}, []shape{shapeHexagon}},
	{"[/", []string{"/]", `\]`}, []shape{shapeLeanRight, shapeTrapezoid}},
	{`[\`, []string{`\]`, "/]"}, []shape{shapeLeanLeft, shapeTrapezoidAlt}},
	{"(", []string{")"}, []shape{shapeRound}},
	{"[", []string{"]"}, []shape{shapeRect}},
	{"{", []string{"}"}, []shape{shapeRhombus}},
	{">", []string{"]"}, []shape{shapeAsym}},
}

func (p *chainParser) shape(n *node) {
	r := p.rest()
	for _, o := range shapeOpeners {
		if !strings.HasPrefix(r, o.open) {
			continue
		}
		body := r[len(o.open):]
		text, after, ok := "", "", false
		trimmed := strings.TrimLeft(body, " ")
		if strings.HasPrefix(trimmed, `"`) {
			if q := strings.Index(trimmed[1:], `"`); q >= 0 {
				text = trimmed[:q+2]
				after = strings.TrimLeft(trimmed[q+2:], " ")
				for i, c := range o.closes {
					if strings.HasPrefix(after, c) {
						n.shape = o.shapes[i]
						p.i += len(r) - len(after) + len(c)
						ok = true
						break
					}
				}
			}
		}
		if !ok {
			best, bi := -1, 0
			for i, c := range o.closes {
				if k := strings.Index(body, c); k >= 0 && (best < 0 || k < best) {
					best, bi = k, i
				}
			}
			if best < 0 {
				continue
			}
			text = body[:best]
			n.shape = o.shapes[bi]
			p.i += len(o.open) + best + len(o.closes[bi])
		}
		n.label = label(text)
		n.defined = true
		return
	}
}

// shapeData reads the newer A@{ shape: …, label: "…" } syntax.
func (p *chainParser) shapeData(n *node, data string) {
	n.defined = true
	for _, kv := range splitTopLevel(data, ',') {
		k, v, _ := strings.Cut(kv, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "label":
			n.label = label(v)
		case "shape":
			n.shape = map[string]shape{
				"rect": shapeRect, "rounded": shapeRound, "stadium": shapeStadium, "pill": shapeStadium,
				"subroutine": shapeSubroutine, "fr-rect": shapeSubroutine, "cyl": shapeCylinder, "database": shapeCylinder, "db": shapeCylinder,
				"circle": shapeCircle, "circ": shapeCircle, "dbl-circ": shapeDoubleCircle, "double-circle": shapeDoubleCircle,
				"diam": shapeRhombus, "diamond": shapeRhombus, "decision": shapeRhombus, "question": shapeRhombus,
				"hex": shapeHexagon, "hexagon": shapeHexagon, "prepare": shapeHexagon,
				"lean-r": shapeLeanRight, "in-out": shapeLeanRight, "lean-l": shapeLeanLeft, "out-in": shapeLeanLeft,
				"trap-b": shapeTrapezoid, "trap-t": shapeTrapezoidAlt, "text": shapeText,
				"odd": shapeAsym, "start": shapeStart, "sm-circ": shapeStart, "stop": shapeEnd, "fr-circ": shapeEnd,
			}[strings.Trim(v, `"`)]
		}
	}
}

func splitTopLevel(s string, sep rune) []string {
	var out []string
	quoted, start := false, 0
	for i, r := range s {
		if r == '"' {
			quoted = !quoted
		}
		if r == sep && !quoted {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// edgeMeta says whether a statement only styles an edge ("e1@{ animate:
// true }"): one about a known edge id, or with only edge keys. Such
// statements aren't nodes.
func edgeMeta(g *graph, st string) bool {
	m := metaStmt.FindStringSubmatch(st)
	if m == nil {
		return false
	}
	if g.edgeIDs[m[1]] {
		return true
	}
	keys := 0
	for _, kv := range splitTopLevel(m[2], ',') {
		k, _, _ := strings.Cut(kv, ":")
		switch strings.TrimSpace(k) {
		case "animate", "animation", "curve":
			keys++
		case "":
		default:
			return false
		}
	}
	return keys > 0
}

// edgeOp reads one edge operator and its label.
func (p *chainParser) edgeOp() (*edge, bool) {
	r := p.rest()
	if m := edgeID.FindString(r); m != "" && len(r) > len(m) && strings.ContainsRune("-=~<ox", rune(r[len(m)])) {
		p.g.edgeIDs[m[:len(m)-1]] = true
		p.i += len(m)
		r = p.rest()
	}
	e := &edge{}
	if m := edgeOpen.FindStringSubmatchIndex(r); m != nil {
		opener := r[m[4]:m[5]]
		body := r[m[1]:]
		if c := closers[opener].FindStringSubmatchIndex(body); c != nil {
			e.label = label(body[:c[0]])
			if m[2] >= 0 {
				e.tail = headOf(r[m[2]:m[3]], true)
			}
			closer := body[c[0]:c[1]]
			e.style, _ = styleOf(opener + closer)
			_, e.minlen = styleOf(closer)
			e.head = headOf(string(closer[len(closer)-1]), false)
			if closer[len(closer)-1] == '-' || closer[len(closer)-1] == '=' {
				e.head = headNone
			}
			p.i += m[1] + c[1]
			p.pipe(e)
			return e, true
		}
	}
	m := edgePlain.FindStringSubmatch(r)
	if m == nil || !p.atEdge() {
		return nil, false
	}
	body := m[2] + m[3] + m[4] + m[5]
	e.style, e.minlen = styleOf(body)
	e.tail = headOf(m[1], true)
	e.head = headOf(m[6], false)
	if m[5] != "" {
		e.style = lineInvisible
	}
	p.i += len(m[0])
	p.pipe(e)
	return e, true
}

func (p *chainParser) pipe(e *edge) {
	if m := pipeLabel.FindStringSubmatch(p.rest()); m != nil {
		e.label = label(m[1])
		p.i += len(m[0])
	}
	if len(e.label) == 1 && e.label[0] == "" {
		e.label = nil
	}
}

func headOf(s string, tail bool) head {
	switch s {
	case ">", "<":
		return headArrow
	case "o":
		return headCircle
	case "x":
		return headCross
	}
	return headNone
}

// styleOf is the line style and minimum length (in ranks) of an operator.
func styleOf(op string) (lineStyle, int) {
	switch {
	case strings.Contains(op, "."):
		return lineDotted, max(1, strings.Count(op, "."))
	case strings.Contains(op, "="):
		return lineThick, max(1, strings.Count(op, "=")-1)
	case strings.Contains(op, "~"):
		return lineInvisible, max(1, strings.Count(op, "~")-2)
	}
	return lineSolid, max(1, strings.Count(op, "-")-1)
}
