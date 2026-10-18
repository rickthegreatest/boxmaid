package renderer

import (
	"regexp"
	"strings"
)

// C4 diagrams (C4Context, C4Container, …) are drawn with the flowchart
// layout: people, systems and containers are boxes with their type and
// description, boundaries are frames, relations labelled edges.

var c4Call = regexp.MustCompile(`^(\w+)\s*\((.*)\)\s*(\{)?\s*$`)

func parseC4(src string) (*graph, string, error) {
	g := newGraph("TB")
	stack := []*cluster{g.root}
	title := ""
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		cur := stack[len(stack)-1]
		switch {
		case line == "":
			continue
		case line == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		case strings.HasPrefix(line, "title "):
			title = strings.TrimSpace(strings.TrimPrefix(line, "title "))
			continue
		}
		m := c4Call.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fn, args := m[1], c4Args(m[2])
		arg := func(i int) string {
			if i < len(args) {
				return args[i]
			}
			return ""
		}
		lower := strings.ToLower(fn)
		switch {
		case strings.HasSuffix(lower, "boundary") || isDeploymentNode(lower):
			t := arg(1)
			if kind := arg(2); kind != "" {
				t += " [" + kind + "]"
			}
			c := g.addCluster(arg(0), t, cur)
			stack = append(stack, c)
			continue
		case strings.HasPrefix(lower, "rel") || strings.HasPrefix(lower, "birel"):
			e := &edge{from: arg(0), to: arg(1), head: headArrow}
			if strings.HasPrefix(lower, "birel") {
				e.tail = headArrow
			}
			if strings.Contains(lower, "back") {
				e.from, e.to = e.to, e.from
			}
			e.label = label(arg(2))
			if t := arg(3); t != "" {
				e.label = append(e.label, "["+t+"]")
			}
			g.edges = append(g.edges, e)
		case strings.HasPrefix(lower, "update") || strings.HasPrefix(lower, "addelementtag") || strings.HasPrefix(lower, "addreltag"):
		default:
			// Person, System, Container, Component and their _Ext/Db/Queue
			// variants: (alias, label, [technology,] description).
			kind := strings.TrimSuffix(strings.TrimSuffix(fn, "_Ext"), "_ext")
			ext := kind != fn
			n := g.node(arg(0), cur)
			n.defined = true
			n.label = label(arg(1))
			tech, desc := "", arg(2)
			if strings.HasPrefix(lower, "container") || strings.HasPrefix(lower, "component") {
				tech, desc = arg(2), arg(3)
			}
			base := strings.TrimSuffix(strings.TrimSuffix(kind, "Db"), "Queue")
			tag := "[" + base
			if tech != "" {
				tag += ": " + tech
			}
			tag += "]"
			n.detail = append([]string{tag}, label(desc)...)
			if desc == "" {
				n.detail = []string{tag}
			}
			n.dashed = ext
			switch {
			case strings.HasSuffix(kind, "Db"):
				n.shape = shapeCylinder
			case strings.HasSuffix(kind, "Queue"), strings.HasPrefix(lower, "person"):
				n.shape = shapeRound
			}
		}
		if m[3] == "{" {
			// Something else opening a block: keep the braces balanced.
			stack = append(stack, cur)
		}
	}
	g.finish()
	if len(g.nodes) == 0 {
		return nil, title, ErrEmpty
	}
	return g, title, nil
}

// isDeploymentNode says whether a call is a deployment node, which frames
// what it contains like a boundary does.
func isDeploymentNode(lower string) bool {
	switch lower {
	case "deployment_node", "deployment_node_l", "deployment_node_r", "node", "node_l", "node_r":
		return true
	}
	return false
}

// c4Args splits call arguments at commas outside quotes, dropping quotes
// and named arguments ($tags="…").
func c4Args(s string) []string {
	var out []string
	for _, a := range splitTopLevel(s, ',') {
		a = strings.TrimSpace(a)
		if strings.HasPrefix(a, "$") {
			continue
		}
		out = append(out, strings.Trim(a, `"`))
	}
	return out
}
