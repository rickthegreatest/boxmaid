package renderer

import (
	"regexp"
	"strings"
)

// Entity relationship diagrams: entities are boxes with their attributes in
// aligned columns (type, name, keys, comment); relationships are edges
// labelled with their verb, with each end's cardinality written as 1, 0..1,
// 0..* or 1..* (crow's feet don't fit in a character cell). Identifying
// relationships (--) are solid, non-identifying ones (..) dotted.

var (
	erRel    = regexp.MustCompile(`^("[^"]+"|[\w\-.]+)\s*(\|o|\|\||\}o|\}\||o\||o\{|\|\{)(--|\.\.)(o\||\|\||o\{|\|\{|\|o|\}o|\}\|)\s*("[^"]+"|[\w\-.]+)\s*:\s*(.*)$`)
	erEntity = regexp.MustCompile(`^("[^"]+"|[\w\-.]+)(?:\[("?)([^\]]*?)("?)\])?\s*(\{)?\s*$`)
	erAttr   = regexp.MustCompile(`^(\S+)\s+(\S+)(?:\s+((?:PK|FK|UK)(?:\s*,\s*(?:PK|FK|UK))*))?(?:\s+"([^"]*)")?\s*$`)
)

func erCardinality(tok string) string {
	switch tok {
	case "||":
		return "1"
	case "|o", "o|":
		return "0..1"
	case "}o", "o{":
		return "0..*"
	case "}|", "|{":
		return "1..*"
	}
	return ""
}

func parseER(_, rest string) (*graph, string, error) {
	g := newGraph("TB")
	attrs := map[string][][4]string{}
	entity := func(id string) *node {
		id = strings.Trim(id, `"`)
		n := g.node(id, g.root)
		n.defined = true
		return n
	}
	open := ""
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if open != "" {
			if line == "}" {
				open = ""
				continue
			}
			if m := erAttr.FindStringSubmatch(line); m != nil {
				attrs[open] = append(attrs[open], [4]string{m[1], m[2], strings.ReplaceAll(m[3], " ", ""), m[4]})
			}
			continue
		}
		if strings.HasPrefix(line, "direction ") {
			g.dir = dirOf(strings.TrimSpace(strings.TrimPrefix(line, "direction ")))
			continue
		}
		if m := erRel.FindStringSubmatch(line); m != nil {
			from, to := entity(m[1]), entity(m[5])
			e := &edge{from: from.id, to: to.id, fromLabel: erCardinality(m[2]), toLabel: erCardinality(m[4])}
			if m[3] == ".." {
				e.style = lineDotted
			}
			e.label = label(strings.Trim(m[6], `"`))
			g.edges = append(g.edges, e)
			continue
		}
		if m := erEntity.FindStringSubmatch(line); m != nil {
			n := entity(m[1])
			if m[3] != "" {
				n.label = []string{m[3]}
			}
			if m[5] == "{" {
				open = n.id
			}
		}
	}
	for id, rows := range attrs {
		n := g.nodes[id]
		if n == nil {
			continue
		}
		var cols [4]int
		for _, r := range rows {
			for i := range r {
				cols[i] = max(cols[i], width(r[i]))
			}
		}
		var lines []string
		for _, r := range rows {
			l := padRight(r[0], cols[0]) + "  " + padRight(r[1], cols[1])
			if cols[2] > 0 {
				l += "  " + padRight(r[2], cols[2])
			}
			if r[3] != "" {
				l += "  " + r[3]
			}
			lines = append(lines, strings.TrimRight(l, " "))
		}
		n.sections = [][]string{lines}
	}
	g.finish()
	if len(g.nodes) == 0 {
		return nil, "", ErrEmpty
	}
	return g, "", nil
}

func padRight(s string, w int) string {
	if n := w - width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
