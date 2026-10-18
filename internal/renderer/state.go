package renderer

import (
	"regexp"
	"strings"
)

// State diagrams are drawn with the flowchart layout: states are rounded
// boxes, [*] a dot (start) or ringed dot (end) per scope, composite states
// frames, <<choice>> a decision and <<fork>>/<<join>> bars.

var (
	stTransition = regexp.MustCompile(`^(\S+?)\s*-->\s*(\S+?)\s*(?::\s*(.*))?$`)
	stAlias      = regexp.MustCompile(`^state\s+"([^"]*)"\s+as\s+(\S+?)\s*(\{)?\s*$`)
	stStereo     = regexp.MustCompile(`^state\s+(\S+?)\s+<<(\w+)>>\s*$`)
	stComposite  = regexp.MustCompile(`^state\s+(\S+?)\s*\{\s*$`)
	stDesc       = regexp.MustCompile(`^([\w.-]+)\s*:\s*(.+)$`)
	stNote       = regexp.MustCompile(`(?i)^note\s+(left|right)\s+of\s+(\S+?)\s*(?::\s*(.*))?$`)
)

func parseState(src string) (*graph, error) {
	g := newGraph("TB")
	stack := []*cluster{g.root}
	notes := 0
	var noteOpen *node
	state := func(id string, c *cluster) *node {
		n := g.node(id, c)
		if !n.defined {
			n.shape = shapeRound
		}
		return n
	}
	// [*] means the start (as a source) or the end (as a target) of the
	// scope it appears in.
	endpoint := func(id string, c *cluster, source bool) string {
		if id != "[*]" {
			state(id, c)
			return id
		}
		key := "\x00start:" + c.id
		sh := shapeStart
		if !source {
			key, sh = "\x00end:"+c.id, shapeEnd
		}
		n := g.node(key, c)
		n.shape, n.defined, n.label = sh, true, []string{""}
		return key
	}
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		cur := stack[len(stack)-1]
		if noteOpen != nil {
			if strings.EqualFold(line, "end note") {
				noteOpen = nil
				continue
			}
			noteOpen.label = append(noteOpen.label, line)
			continue
		}
		switch {
		case line == "" || line == "--" || strings.HasPrefix(line, "hide ") || strings.HasPrefix(line, "scale ") ||
			strings.HasPrefix(line, "classDef ") || strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "accTitle") || strings.HasPrefix(line, "accDescr"):
			continue
		case line == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		case strings.HasPrefix(line, "direction "):
			d := dirOf(strings.TrimSpace(strings.TrimPrefix(line, "direction ")))
			if cur == g.root {
				g.dir = d
			} else {
				cur.dir = d
			}
			continue
		}
		if m := stAlias.FindStringSubmatch(line); m != nil {
			if m[3] == "{" {
				stack = append(stack, g.addCluster(m[2], m[1], cur))
				continue
			}
			n := state(m[2], cur)
			n.label, n.defined = label(m[1]), true
			continue
		}
		if m := stStereo.FindStringSubmatch(line); m != nil {
			n := state(m[1], cur)
			n.defined = true
			switch strings.ToLower(m[2]) {
			case "choice":
				n.shape, n.label = shapeRhombus, []string{""}
			case "fork", "join":
				n.shape, n.label = shapeBar, []string{""}
			}
			continue
		}
		if m := stComposite.FindStringSubmatch(line); m != nil {
			stack = append(stack, g.addCluster(m[1], m[1], cur))
			continue
		}
		if m := stNote.FindStringSubmatch(line); m != nil {
			notes++
			id := "\x00note" + string(rune('a'+notes))
			n := g.node(id, cur)
			n.shape, n.dashed, n.defined = shapeNote, true, true
			n.label = nil
			if m[3] != "" {
				n.label = label(m[3])
			} else {
				noteOpen = n
			}
			target := state(m[2], cur).id
			e := &edge{from: target, to: id, style: lineDotted}
			if strings.EqualFold(m[1], "left") {
				e.from, e.to = id, target
			}
			g.edges = append(g.edges, e)
			continue
		}
		if m := stTransition.FindStringSubmatch(line); m != nil {
			from := endpoint(m[1], cur, true)
			to := endpoint(m[2], cur, false)
			e := &edge{from: from, to: to, head: headArrow}
			if m[3] != "" {
				e.label = label(m[3])
			}
			g.edges = append(g.edges, e)
			continue
		}
		if m := stDesc.FindStringSubmatch(line); m != nil {
			n := state(m[1], cur)
			if !n.defined {
				n.label = nil
			}
			n.defined = true
			n.label = append(n.label, label(m[2])...)
			continue
		}
		if !strings.ContainsAny(line, " \t") {
			state(line, cur)
		}
	}
	// A composite state named by a transition is the frame, not a node.
	g.finish()
	if len(g.nodes) == 0 {
		return nil, ErrEmpty
	}
	return g, nil
}
