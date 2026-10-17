package boxmaid

import (
	"regexp"
	"strings"
)

// Class diagrams: classes are boxes with their «annotation», name,
// attributes and methods; relations are edges with UML heads (hollow
// triangle for inheritance and realisation, filled diamond for composition,
// hollow diamond for aggregation, arrow for association and dependency),
// labels and multiplicities. Namespaces are frames.

var (
	classRel = regexp.MustCompile(`^(\S+?)\s*(?:"([^"]*)")?\s*` +
		`(<\|--|<\|\.\.|\*--|\*\.\.|o--|o\.\.|<--|<\.\.|--\|>|\.\.\|>|--\*|\.\.\*|--o|\.\.o|-->|\.\.>|--|\.\.|\(\)--|--\(\))` +
		`\s*(?:"([^"]*)")?\s*(\S+?)\s*(?::\s*(.*))?$`)
	classDecl   = regexp.MustCompile(`^class\s+([^\s{~\[:]+)(~[^~]+~)?(?:\["([^"]*)"\])?(?::::\S+)?\s*(\{)?\s*(\})?$`)
	classMember = regexp.MustCompile(`^([^\s:]+)\s*:\s*(.+)$`)
	classAnnot  = regexp.MustCompile(`^<<\s*([^>]+?)\s*>>\s*(\S+)?$`)
	classNote   = regexp.MustCompile(`^note\s+(?:for\s+(\S+)\s+)?"(.*)"$`)
	generic     = regexp.MustCompile(`~([^~]+)~`)
)

type classInfo struct {
	attrs, methods []string
	annotations    []string
}

func parseClass(_, rest string) (*graph, string, error) {
	g := newGraph("TB")
	stack := []*cluster{g.root}
	info := map[string]*classInfo{}
	notes := 0
	class := func(id string, c *cluster) (*node, *classInfo) {
		id = strings.TrimSpace(id)
		disp := generic.ReplaceAllString(id, "<$1>")
		id = generic.ReplaceAllString(id, "")
		n := g.node(id, c)
		if !n.defined {
			n.defined = true
			n.label = []string{disp}
		}
		if info[id] == nil {
			info[id] = &classInfo{}
		}
		return n, info[id]
	}
	var open *classInfo // inside "class X {"
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		cur := stack[len(stack)-1]
		if open != nil {
			if line == "}" {
				open = nil
				continue
			}
			if m := classAnnot.FindStringSubmatch(line); m != nil && m[2] == "" {
				open.annotations = append(open.annotations, m[1])
				continue
			}
			addMember(open, line)
			continue
		}
		switch {
		case strings.HasPrefix(line, "direction "):
			g.dir = dirOf(strings.TrimSpace(strings.TrimPrefix(line, "direction ")))
			continue
		case strings.HasPrefix(line, "namespace "):
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "namespace "), "{"))
			stack = append(stack, g.addCluster(name, name, cur))
			continue
		case line == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		case strings.HasPrefix(line, "classDef ") || strings.HasPrefix(line, "cssClass ") || strings.HasPrefix(line, "style ") ||
			strings.HasPrefix(line, "click ") || strings.HasPrefix(line, "link ") || strings.HasPrefix(line, "callback ") ||
			strings.HasPrefix(line, "accTitle") || strings.HasPrefix(line, "accDescr"):
			continue
		}
		if m := classDecl.FindStringSubmatch(line); m != nil {
			n, ci := class(m[1]+m[2], cur)
			if m[3] != "" {
				n.label = []string{m[3]}
			}
			if m[4] == "{" && m[5] == "" {
				open = ci
			}
			continue
		}
		if m := classAnnot.FindStringSubmatch(line); m != nil && m[2] != "" {
			_, ci := class(m[2], cur)
			ci.annotations = append(ci.annotations, m[1])
			continue
		}
		if m := classNote.FindStringSubmatch(line); m != nil {
			notes++
			id := "\x00note" + itoa(notes)
			n := g.node(id, cur)
			n.shape, n.dashed, n.defined, n.label = shapeNote, true, true, label(m[2])
			if m[1] != "" {
				class(m[1], cur)
				g.edges = append(g.edges, &edge{from: id, to: m[1], style: lineDotted})
			}
			continue
		}
		if m := classRel.FindStringSubmatch(line); m != nil {
			class(m[1], cur)
			class(m[5], cur)
			e := &edge{from: generic.ReplaceAllString(m[1], ""), to: generic.ReplaceAllString(m[5], ""),
				fromLabel: m[2], toLabel: m[4]}
			op := m[3]
			if strings.Contains(op, "..") {
				e.style = lineDotted
			}
			e.tail = classHead(op[:2])
			e.head = classHead(op[len(op)-2:])
			if m[6] != "" {
				e.label = label(m[6])
			}
			g.edges = append(g.edges, e)
			continue
		}
		if m := classMember.FindStringSubmatch(line); m != nil {
			_, ci := class(m[1], cur)
			addMember(ci, m[2])
			continue
		}
		if !strings.ContainsAny(line, " \t") {
			class(line, cur)
		}
	}
	for id, ci := range info {
		n := g.nodes[id]
		if n == nil {
			continue
		}
		for _, a := range ci.annotations {
			n.header = append(n.header, "«"+a+"»")
		}
		if len(ci.attrs) > 0 || len(ci.methods) > 0 {
			n.sections = [][]string{ci.attrs, ci.methods}
		}
	}
	g.finish()
	if len(g.nodes) == 0 {
		return nil, "", ErrEmpty
	}
	return g, "", nil
}

// addMember files a member as an attribute or (with parentheses) a method,
// showing generics as <T> and dropping the static/abstract markers.
func addMember(ci *classInfo, m string) {
	m = strings.TrimSpace(generic.ReplaceAllString(m, "<$1>"))
	if m == "" {
		return
	}
	if strings.Contains(m, "(") {
		ci.methods = append(ci.methods, strings.TrimRight(m, "$*"))
	} else {
		ci.attrs = append(ci.attrs, strings.TrimRight(m, "$*"))
	}
}

func classHead(tok string) head {
	switch tok {
	case "<|", "|>":
		return headTriangle
	case "*-", "-*":
		return headDiamond
	case "o-", "-o", "o.", ".o":
		return headDiamondOpen
	case "<-", "<.", "->", ".>":
		return headArrow
	case "()":
		return headCircle
	}
	if strings.HasSuffix(tok, "*") || strings.HasPrefix(tok, "*") {
		return headDiamond
	}
	return headNone
}
