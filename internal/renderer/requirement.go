package renderer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Requirement diagrams, architecture diagrams and block diagrams, drawn
// with the graph layout.

var (
	reqBlock  = regexp.MustCompile(`^(requirement|functionalRequirement|interfaceRequirement|performanceRequirement|physicalRequirement|designConstraint|element)\s+(\S+?)\s*\{\s*$`)
	reqField  = regexp.MustCompile(`^(\w+)\s*:\s*(.+)$`)
	reqRelFwd = regexp.MustCompile(`^(\S+)\s+-\s*(\w+)\s*->\s+(\S+)$`)
	reqRelBwd = regexp.MustCompile(`^(\S+)\s+<-\s*(\w+)\s*-\s+(\S+)$`)
)

var reqTypeNames = map[string]string{
	"requirement": "Requirement", "functionalRequirement": "Functional Requirement",
	"interfaceRequirement": "Interface Requirement", "performanceRequirement": "Performance Requirement",
	"physicalRequirement": "Physical Requirement", "designConstraint": "Design Constraint", "element": "Element",
}

var reqFieldNames = map[string]string{
	"id": "Id", "text": "Text", "risk": "Risk", "verifymethod": "Verification", "type": "Type", "docref": "Doc ref",
}

func parseRequirement(_, rest string) (*graph, string, error) {
	g := newGraph("TB")
	var open *node
	var fields []string
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if open != nil {
			if line == "}" {
				open.sections = [][]string{fields}
				open, fields = nil, nil
				continue
			}
			if m := reqField.FindStringSubmatch(line); m != nil {
				name := reqFieldNames[strings.ToLower(m[1])]
				if name == "" {
					name = m[1]
				}
				v := strings.Trim(strings.TrimSpace(m[2]), `"`)
				if name == "Risk" || name == "Verification" {
					if r, size := utf8.DecodeRuneInString(v); size > 0 {
						v = string(unicode.ToUpper(r)) + v[size:]
					}
				}
				fields = append(fields, name+": "+v)
			}
			continue
		}
		if strings.HasPrefix(line, "direction ") {
			g.dir = dirOf(strings.TrimSpace(strings.TrimPrefix(line, "direction ")))
			continue
		}
		if m := reqBlock.FindStringSubmatch(line); m != nil {
			n := g.node(m[2], g.root)
			n.defined = true
			n.label = []string{m[2]}
			n.header = []string{"«" + reqTypeNames[m[1]] + "»"}
			open = n
			continue
		}
		var from, to, kind string
		if m := reqRelFwd.FindStringSubmatch(line); m != nil {
			from, kind, to = m[1], m[2], m[3]
		} else if m := reqRelBwd.FindStringSubmatch(line); m != nil {
			to, kind, from = m[1], m[2], m[3]
		} else {
			continue
		}
		g.node(from, g.root)
		g.node(to, g.root)
		g.edges = append(g.edges, &edge{from: from, to: to, head: headArrow, label: []string{"«" + kind + "»"}})
	}
	g.finish()
	if len(g.nodes) == 0 {
		return nil, "", ErrEmpty
	}
	return g, "", nil
}

var (
	archGroup    = regexp.MustCompile(`^group\s+([\w-]+)(?:\(([^)]*)\))?(?:\[([^\]]*)\])?(?:\s+in\s+([\w-]+))?$`)
	archService  = regexp.MustCompile(`^service\s+([\w-]+)(?:\(([^)]*)\))?(?:\[([^\]]*)\])?(?:\s+in\s+([\w-]+))?$`)
	archJunction = regexp.MustCompile(`^junction\s+([\w-]+)(?:\s+in\s+([\w-]+))?$`)
	archEdge     = regexp.MustCompile(`^([\w-]+)(?:\{group\})?\s*:\s*([LRTB])\s*(<?)--(>?)\s*([LRTB])\s*:\s*([\w-]+)(?:\{group\})?$`)
)

func parseArchitecture(_, rest string) (*graph, string, error) {
	g := newGraph("TB")
	horizontal, vertical := 0, 0
	type pending struct{ id, parent string }
	var groups []string
	parents := map[string]string{}
	var members []pending
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
		case archGroup.MatchString(line):
			m := archGroup.FindStringSubmatch(line)
			title := orStr(m[3], m[1])
			if _, exists := g.clusters[m[1]]; !exists {
				groups = append(groups, m[1])
				g.clusters[m[1]] = &cluster{id: m[1]}
			}
			g.clusters[m[1]].title = title
			parents[m[1]] = m[4]
		case archService.MatchString(line):
			m := archService.FindStringSubmatch(line)
			n := g.node(m[1], g.root)
			n.defined = true
			n.label = []string{orStr(m[3], m[1])}
			n.shape, n.header = shapeRect, nil
			if m[2] != "" {
				n.header = []string{m[2]}
			}
			switch m[2] {
			case "database":
				n.shape = shapeCylinder
				n.header = nil
			case "internet", "cloud":
				n.shape = shapeRound
			}
			members = append(members, pending{m[1], m[4]})
		case archJunction.MatchString(line):
			m := archJunction.FindStringSubmatch(line)
			n := g.node(m[1], g.root)
			n.defined, n.shape, n.label = true, shapeStart, []string{""}
			n.header = nil
			members = append(members, pending{m[1], m[2]})
		case archEdge.MatchString(line):
			m := archEdge.FindStringSubmatch(line)
			e := &edge{from: m[1], to: m[6]}
			if m[4] == ">" {
				e.head = headArrow
			}
			if m[3] == "<" {
				e.tail = headArrow
			}
			if (m[2] == "L" || m[2] == "R") && (m[5] == "L" || m[5] == "R") {
				horizontal++
			} else {
				vertical++
			}
			g.edges = append(g.edges, e)
		}
	}
	// Nest groups and place services now that every group is known.
	for _, id := range groups {
		c := g.clusters[id]
		parent := g.root
		if p, ok := g.clusters[parents[id]]; ok && parents[id] != "" && !within(p, c) {
			parent = p
		}
		c.parent = parent
		parent.children = append(parent.children, c)
	}
	for _, m := range members {
		c := g.root
		if parent, ok := g.clusters[m.parent]; ok && m.parent != "" {
			c = parent
		}
		g.moveNode(m.id, c)
	}
	if horizontal > vertical {
		g.dir = "LR"
	}
	g.finish()
	if len(g.nodes) == 0 {
		return nil, "", ErrEmpty
	}
	return g, "", nil
}

// within says whether c is p or one of its ancestors so far, which would
// make putting p inside c a cycle ("group a in b", "group b in a").
func within(p, c *cluster) bool {
	for cur := p; cur != nil; cur = cur.parent {
		if cur == c {
			return true
		}
	}
	return false
}
