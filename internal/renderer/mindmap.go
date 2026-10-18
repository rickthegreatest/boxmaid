package renderer

import (
	"regexp"
	"strings"
)

// Mind maps: the root in a box, its branches as a tree below it. Shapes
// (square, round, circle, bang, cloud, hexagon) only change how a node is
// written down in Mermaid; in a tree the text is what matters, so they're
// shown as plain text, the root boxed.
//
//	╭─────────╮
//	│ mindmap │
//	╰────┬────╯
//	     ├── Origins
//	     │   └── Long history
//	     └── Research

type mindNode struct {
	text     []string
	indent   int
	children []*mindNode
}

var mindShape = regexp.MustCompile(`^[^\[\(\{\)]*?(\(\(|\(-|\)\)|\(|\[|\{\{|\))(.*?)(\)\)|-\)|\(\(|\)|\]|\}\}|\()$`)

func mindText(s string) []string {
	s = strings.TrimSpace(s)
	if m := mindShape.FindStringSubmatch(s); m != nil {
		s = m[2]
	}
	return label(s)
}

func renderMindmap(_, rest string, opt Options) (*Picture, string, error) {
	var root *mindNode
	var stack []*mindNode
	for _, raw := range strings.Split(rest, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(t, "::icon(") || strings.HasPrefix(t, ":::") {
			continue
		}
		if i := strings.Index(t, ":::"); i > 0 {
			t = strings.TrimSpace(t[:i])
		}
		n := &mindNode{text: mindText(t), indent: indent}
		if root == nil {
			root, stack = n, []*mindNode{n}
			continue
		}
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, n)
		stack = append(stack, n)
	}
	if root == nil {
		return nil, "", ErrEmpty
	}
	textW := 60
	if opt.Width > 0 {
		textW = max(12, opt.Width-8)
	}
	// Lay out the tree as lines first, then draw.
	type line struct {
		prefix []string // tree glyphs before the text, one per level
		text   string
		depth  int
	}
	var lines []line
	var walk func(n *mindNode, prefix []string, depth int)
	walk = func(n *mindNode, prefix []string, depth int) {
		for i, c := range n.children {
			lastChild := i == len(n.children)-1
			branch, cont := "├── ", "│   "
			if lastChild {
				branch, cont = "└── ", "    "
			}
			text := wrap(c.text, max(8, textW-4*depth))
			for j, t := range text {
				p := append(append([]string(nil), prefix...), branch)
				if j > 0 {
					p[len(p)-1] = cont
					if len(c.children) > 0 {
						p = append(p[:len(p)-1], cont)
					}
				}
				lines = append(lines, line{p, t, depth})
			}
			walk(c, append(append([]string(nil), prefix...), cont), depth+1)
		}
	}
	walk(root, nil, 1)
	rootText := wrap(root.text, textW)
	boxW := maxWidth(rootText) + 4
	indent := boxW / 2
	cv := newCanvas(indent+textW+boxW+8, len(rootText)+len(lines)+3, opt.ASCII)
	cv.box(0, 0, boxW, len(rootText)+2, lineSolid, true, false, RoleNode, "")
	for i, t := range rootText {
		cv.text(2, 1+i, t, RoleNodeText, "", true)
	}
	if len(lines) > 0 {
		cv.addBits(indent, len(rootText)+1, bitDown, lineSolid, RoleNode, "")
	}
	y := len(rootText) + 2
	for _, l := range lines {
		x := indent
		for _, p := range l.prefix {
			// Tree glyphs as line pieces, so they join cleanly.
			switch p {
			case "├── ":
				cv.vline(x, y-1, y+1, lineSolid, RoleEdge, "")
				cv.hline(x, x+2, y, lineSolid, RoleEdge, "")
			case "└── ":
				cv.vline(x, y-1, y, lineSolid, RoleEdge, "")
				cv.hline(x, x+2, y, lineSolid, RoleEdge, "")
			case "│   ":
				cv.vline(x, y-1, y+1, lineSolid, RoleEdge, "")
			}
			x += 4
		}
		role := RoleNone
		bold := false
		if l.depth == 1 {
			role, bold = RoleNodeText, true
		}
		cv.text(x, y, l.text, role, "", bold)
		y++
	}
	return cv.picture(), "", nil
}
