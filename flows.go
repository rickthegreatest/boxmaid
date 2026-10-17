package boxmaid

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Sankey diagrams, radar charts and treemaps: in a terminal these read best
// as proportional bars and nested boxes.

func fmtNum(v float64) string { return formatNumber(v) }

// renderSankey lists each node's outflows, in flow order (sources first),
// as bars proportional to their value: the widths of a Sankey diagram,
// side by side.
func renderSankey(_, rest string, opt Options) (*Picture, string, error) {
	type flow struct {
		from, to string
		v        float64
	}
	var flows []flow
	order := map[string]int{}
	seen := func(n string) {
		if _, ok := order[n]; !ok {
			order[n] = len(order)
		}
	}
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		cols := splitCSV(line)
		if len(cols) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(cols[2]), 64)
		if err != nil {
			continue
		}
		f := flow{strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1]), v}
		seen(f.from)
		seen(f.to)
		flows = append(flows, f)
	}
	if len(flows) == 0 {
		return nil, "", ErrEmpty
	}
	// Depth of each node: sources at 0, each flow one step further.
	depth := map[string]int{}
	for range len(order) {
		for _, f := range flows {
			depth[f.to] = max(depth[f.to], depth[f.from]+1)
		}
	}
	var sources []string
	out := map[string][]flow{}
	total := map[string]float64{}
	maxV := 0.0
	for _, f := range flows {
		if len(out[f.from]) == 0 {
			sources = append(sources, f.from)
		}
		out[f.from] = append(out[f.from], f)
		total[f.from] += f.v
		maxV = math.Max(maxV, f.v)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		if depth[sources[i]] != depth[sources[j]] {
			return depth[sources[i]] < depth[sources[j]]
		}
		return order[sources[i]] < order[sources[j]]
	})
	nameW := 0
	for n := range order {
		nameW = max(nameW, width(n))
	}
	nameW = min(nameW, 28)
	barW := 30
	if opt.Width > 0 {
		barW = max(8, min(40, opt.Width-2*nameW-16))
	}
	rows := 0
	for _, s := range sources {
		rows += len(out[s]) + 2
	}
	cv := newCanvas(nameW*2+barW+20, rows+1, opt.ASCII)
	y := 0
	for i, s := range sources {
		cv.text(0, y, truncate(s, nameW), RoleNodeText, gitColors[i%len(gitColors)], true)
		cv.text(nameW+2, y, fmtNum(total[s]), RoleMuted, "", false)
		y++
		for _, f := range out[s] {
			units := int(f.v/maxV*float64(barW)*8 + 0.5)
			bar := strings.Repeat("█", units/8) + []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}[units%8]
			if opt.ASCII {
				bar = strings.Repeat("#", max(1, (units+4)/8))
			}
			if bar == "" {
				bar = "▏"
			}
			cv.text(2, y, bar, RoleBar, gitColors[i%len(gitColors)], false)
			cv.text(3+width(bar), y, fmtNum(f.v)+" → "+f.to, RoleNone, "", false)
			y++
		}
		y++
	}
	return cv.picture(), "", nil
}

// splitCSV splits one CSV line, honouring double quotes.
func splitCSV(line string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"' && quoted && i+1 < len(line) && line[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == ',' && !quoted:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(out, cur.String())
}

var (
	radarAxis  = regexp.MustCompile(`^axis\s+(.+)$`)
	radarCurve = regexp.MustCompile(`^curve\s+([\w-]+)(?:\["([^"]*)"\])?\s*\{(.*)\}\s*$`)
	radarItem  = regexp.MustCompile(`^([\w-]+)(?:\["([^"]*)"\])?$`)
)

// renderRadar shows each axis with every curve's value as a bar, the
// values a radar chart plots around its spokes.
func renderRadar(_, rest string, opt Options) (*Picture, string, error) {
	type item struct{ id, name string }
	var axes, curves []item
	var values [][]float64
	title := ""
	maxV := 0.0
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "title "):
			title = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "title")), `"`)
		case radarAxis.MatchString(line):
			for _, a := range splitTopLevel(radarAxis.FindStringSubmatch(line)[1], ',') {
				if m := radarItem.FindStringSubmatch(strings.TrimSpace(a)); m != nil {
					axes = append(axes, item{m[1], orStr(m[2], m[1])})
				}
			}
		case strings.HasPrefix(line, "max "):
			maxV, _ = strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "max")), 64)
		case radarCurve.MatchString(line):
			m := radarCurve.FindStringSubmatch(line)
			curves = append(curves, item{m[1], orStr(m[2], m[1])})
			vals := make([]float64, len(axes))
			for i, v := range splitTopLevel(m[3], ',') {
				v = strings.TrimSpace(v)
				if k, val, ok := strings.Cut(v, ":"); ok { // axis: value
					for j, a := range axes {
						if a.id == strings.TrimSpace(k) {
							vals[j], _ = strconv.ParseFloat(strings.TrimSpace(val), 64)
						}
					}
				} else if i < len(vals) {
					vals[i], _ = strconv.ParseFloat(v, 64)
				}
			}
			values = append(values, vals)
		}
	}
	if len(axes) == 0 || len(curves) == 0 {
		return nil, title, ErrEmpty
	}
	if maxV == 0 {
		for _, vs := range values {
			for _, v := range vs {
				maxV = math.Max(maxV, v)
			}
		}
	}
	maxV = math.Max(maxV, 1)
	axisW, curveW := 0, 0
	for _, a := range axes {
		axisW = max(axisW, width(a.name))
	}
	for _, c := range curves {
		curveW = max(curveW, width(c.name))
	}
	barW := 24
	if opt.Width > 0 {
		barW = max(6, min(30, opt.Width-axisW-curveW-12))
	}
	cv := newCanvas(axisW+curveW+barW+16, len(axes)*(len(curves)+1)+1, opt.ASCII)
	y := 0
	for ai, a := range axes {
		cv.text(0, y, a.name, RoleNodeText, "", true)
		for ci, c := range curves {
			v := values[ci][ai]
			n := int(math.Max(0, math.Min(1, v/maxV))*float64(barW) + 0.5)
			cv.text(axisW+2, y, padRight(c.name, curveW), RoleMuted, gitColors[ci%len(gitColors)], false)
			bar := strings.Repeat("█", n)
			if opt.ASCII {
				bar = strings.Repeat("#", n)
			}
			cv.text(axisW+curveW+4, y, bar, RoleBar, gitColors[ci%len(gitColors)], false)
			cv.text(axisW+curveW+5+n, y, fmtNum(v), RoleMuted, "", false)
			y++
		}
		y++
	}
	return cv.picture(), title, nil
}

// renderTreemap draws the hierarchy as nested boxes whose areas follow the
// values, cut alternately across and down (slice and dice).
func renderTreemap(_, rest string, opt Options) (*Picture, string, error) {
	type tnode struct {
		name     string
		value    float64
		indent   int
		children []*tnode
	}
	root := &tnode{indent: -1}
	stack := []*tnode{root}
	title := ""
	for _, raw := range strings.Split(rest, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(t, "title ") {
			title = strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "title")), `"`)
			continue
		}
		if strings.HasPrefix(t, "classDef ") || strings.HasPrefix(t, "%%") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		name, val, hasVal := strings.Cut(t, ":")
		if i := strings.Index(val, ":::"); i >= 0 {
			val = val[:i]
		}
		n := &tnode{name: strings.Trim(strings.TrimSpace(name), `"`), indent: indent}
		if hasVal {
			n.value, _ = strconv.ParseFloat(strings.TrimSpace(val), 64)
		}
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		p := stack[len(stack)-1]
		p.children = append(p.children, n)
		stack = append(stack, n)
	}
	var sum func(n *tnode) float64
	sum = func(n *tnode) float64 {
		if len(n.children) == 0 {
			return math.Max(n.value, 0)
		}
		n.value = 0
		for _, c := range n.children {
			n.value += sum(c)
		}
		return n.value
	}
	if len(root.children) == 0 || sum(root) <= 0 {
		return nil, title, ErrEmpty
	}
	W := 80
	if opt.Width > 0 {
		W = max(20, opt.Width)
	}
	H := max(8, W/4) // cells are about twice as tall as wide
	cv := newCanvas(W, H, opt.ASCII)
	var place func(nodes []*tnode, x, y, w, h int, across bool, depth int)
	place = func(nodes []*tnode, x, y, w, h int, across bool, depth int) {
		total := 0.0
		for _, n := range nodes {
			total += n.value
		}
		if total <= 0 || w < 3 || h < 2 {
			return
		}
		pos := 0.0
		for i, n := range nodes {
			share := n.value / total
			var bx, by, bw, bh int
			if across {
				bx = x + int(pos*float64(w)+0.5)
				end := x + int((pos+share)*float64(w)+0.5)
				if i == len(nodes)-1 {
					end = x + w
				}
				by, bw, bh = y, end-bx, h
			} else {
				by = y + int(pos*float64(h)+0.5)
				end := y + int((pos+share)*float64(h)+0.5)
				if i == len(nodes)-1 {
					end = y + h
				}
				bx, bw, bh = x, w, end-by
			}
			pos += share
			if bw < 2 || bh < 2 {
				continue
			}
			role := RoleNode
			if len(n.children) > 0 {
				role = RoleCluster
			}
			cv.box(bx, by, bw, bh, lineSolid, len(n.children) > 0, false, role, gitColors[depth%len(gitColors)])
			lbl := n.name
			if len(n.children) == 0 {
				lbl += " " + fmtNum(n.value)
			}
			if bw > 4 {
				cv.text(bx+1, by, truncate(" "+lbl+" ", bw-2), RoleClusterTitle, "", len(n.children) > 0)
			}
			if len(n.children) > 0 {
				place(n.children, bx+1, by+1, bw-2, bh-2, !across, depth+1)
			}
		}
	}
	place(root.children, 0, 0, W, H, true, 0)
	return cv.picture(), title, nil
}

func orStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
