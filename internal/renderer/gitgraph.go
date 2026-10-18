package renderer

import (
	"regexp"
	"strconv"
	"strings"
)

// Git graphs: each branch is a lane, commits sit along it in order, a new
// branch leaves its parent where it was created and a merge comes back in.
//
//	main     ●───●───────────◉───●
//	             │           │
//	develop      ╰───●───●───╯
//
// Commits are ● (■ highlighted, ⊗ reverted, ◎ cherry-picked, ◉ merges);
// tags are written above them. "gitGraph TB:" runs top to bottom.

type gitCommit struct {
	id, tag, kind string
	branch        int
	parents       []int
	merge, cherry bool
}

type gitBranch struct {
	name  string
	order int
	head  int // the last commit on it (or the one it branched from), -1 before any
	first int // its first own commit, -1 when none
	seen  int // creation order
}

var gitOpt = regexp.MustCompile(`(\w+)\s*:\s*("[^"]*"|\S+)`)

var gitColors = []string{"#4f9cff", "#3ddc84", "#ffb547", "#d58cff", "#ff6b6b", "#4fd6d6", "#ff8fc7", "#a3e635"}

func renderGitGraph(body, rest string, opt Options) (*Picture, string, error) {
	dir := "LR"
	if h := strings.Fields(strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]); len(h) > 1 {
		switch strings.TrimSuffix(h[1], ":") {
		case "TB":
			dir = "TB"
		case "BT":
			dir = "BT"
		}
	}
	branches := []*gitBranch{{name: "main", head: -1, first: -1}}
	byName := map[string]int{"main": 0}
	cur := 0
	var commits []*gitCommit
	opts := func(s string) map[string]string {
		m := map[string]string{}
		for _, o := range gitOpt.FindAllStringSubmatch(s, -1) {
			m[o[1]] = strings.Trim(o[2], `"`)
		}
		return m
	}
	add := func(c *gitCommit) {
		c.branch = cur
		b := branches[cur]
		if b.head >= 0 {
			c.parents = append([]int{b.head}, c.parents...)
		}
		commits = append(commits, c)
		b.head = len(commits) - 1
		if b.first < 0 {
			b.first = b.head
		}
	}
	for _, raw := range strings.Split(rest, "\n") {
		line := strings.TrimSpace(raw)
		word, args, _ := strings.Cut(line, " ")
		o := opts(args)
		switch word {
		case "commit":
			add(&gitCommit{id: o["id"], tag: o["tag"], kind: strings.ToUpper(o["type"])})
		case "branch":
			name := strings.Fields(args + " ")[0]
			if _, ok := byName[name]; ok || name == "" {
				continue
			}
			order := len(branches)
			if v, err := strconv.Atoi(o["order"]); err == nil {
				order = v
			}
			branches = append(branches, &gitBranch{name: name, order: order, head: branches[cur].head, first: -1, seen: len(branches)})
			byName[name] = len(branches) - 1
			cur = len(branches) - 1
		case "checkout", "switch":
			if i, ok := byName[strings.TrimSpace(args)]; ok {
				cur = i
			}
		case "merge":
			name := strings.Fields(args + " ")[0]
			i, ok := byName[name]
			if !ok || branches[i].head < 0 {
				continue
			}
			add(&gitCommit{id: o["id"], tag: o["tag"], kind: strings.ToUpper(o["type"]), merge: true, parents: []int{branches[i].head}})
		case "cherry-pick":
			c := &gitCommit{tag: o["tag"], cherry: true}
			if o["id"] != "" {
				c.tag = strings.TrimSpace(c.tag + " ← " + o["id"])
			}
			add(c)
		}
	}
	if len(commits) == 0 {
		return nil, "", ErrEmpty
	}
	// Lanes in order: main first, then by "order:" and creation.
	lanes := make([]int, len(branches)) // branch → lane
	idx := make([]int, len(branches))
	for i := range idx {
		idx[i] = i
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 1 && (branches[idx[j]].order < branches[idx[j-1]].order); j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	for lane, b := range idx {
		lanes[b] = lane
	}
	nameW := 0
	for _, b := range branches {
		nameW = max(nameW, width(b.name))
	}
	// Geometry: t runs along the history, lane across it.
	const step = 4
	tb := dir != "LR"
	laneGap := 3
	if tb {
		laneGap = max(4, nameW+2)
	}
	pos := func(t, lane int) (int, int) {
		if tb {
			return 1 + lane*laneGap, 2 + t*2
		}
		return nameW + 3 + t*step, 1 + lane*laneGap
	}
	var w, h int
	if tb {
		w, h = 2+len(branches)*laneGap, 4+len(commits)*2
	} else {
		w, h = nameW+6+len(commits)*step, 3+len(branches)*laneGap
	}
	cv := newCanvas(w, h, opt.ASCII)
	line := func(t1, l1, t2, l2 int, color string) {
		x1, y1 := pos(t1, l1)
		x2, y2 := pos(t2, l2)
		if x1 == x2 {
			cv.vline(x1, y1, y2, lineSolid, RoleEdge, color)
		} else {
			cv.hline(x1, x2, y1, lineSolid, RoleEdge, color)
		}
	}
	color := func(b int) string { return gitColors[lanes[b]%len(gitColors)] }
	// Branch lines, forks and merges (forks and merges turn where the lane
	// changes, so they read as curves).
	for bi, b := range branches {
		if b.first < 0 {
			continue
		}
		lane := lanes[bi]
		last := b.first
		for i, c := range commits {
			if c.branch == bi {
				last = i
			}
		}
		line(b.first, lane, last, lane, color(bi))
		if p := commits[b.first].parents; len(p) > 0 {
			from := p[0]
			pl := lanes[commits[from].branch]
			line(from, pl, from, lane, color(bi))
			line(from, lane, b.first, lane, color(bi))
			x, y := pos(from, lane)
			if c := cv.at(x, y); c != nil {
				c.round = true
			}
		}
	}
	for i, c := range commits {
		if !c.merge || len(c.parents) < 2 {
			continue
		}
		src := c.parents[1]
		sl, ml := lanes[commits[src].branch], lanes[c.branch]
		line(src, sl, i, sl, color(commits[src].branch))
		line(i, sl, i, ml, color(commits[src].branch))
		x, y := pos(i, sl)
		if cell := cv.at(x, y); cell != nil {
			cell.round = true
		}
	}
	// Branch names, commits, tags.
	for bi, b := range branches {
		x, y := pos(0, lanes[bi])
		if tb {
			cv.text(max(0, x-width(b.name)/2), 0, b.name, RoleClusterTitle, color(bi), true)
		} else {
			cv.text(0, y, b.name, RoleClusterTitle, color(bi), true)
		}
		_ = x
	}
	for i, c := range commits {
		x, y := pos(i, lanes[c.branch])
		g := "●"
		switch {
		case c.merge:
			g = "◉"
		case c.cherry:
			g = "◎"
		case c.kind == "HIGHLIGHT":
			g = "■"
		case c.kind == "REVERSE":
			g = "⊗"
		}
		cv.text(x, y, g, RoleBar, color(c.branch), false)
		label := c.tag
		if label == "" && c.id != "" {
			label = c.id
		}
		if label == "" {
			continue
		}
		role := RoleMuted
		if c.tag != "" {
			role = RoleAccent
		}
		if tb {
			if cv.blank(x+2, y, width(label)) {
				cv.text(x+2, y, label, role, "", false)
			}
		} else if lx := x - width(label)/2; cv.blank(lx-1, y-1, width(label)+2) {
			cv.text(lx, y-1, label, role, "", false)
		}
	}
	p := cv.picture()
	if dir == "BT" {
		p.flipVertical()
	}
	return p, "", nil
}

// flipVertical turns a picture upside down, mirroring line glyphs so they
// still join.
func (p *Picture) flipVertical() {
	p.materialize()
	for i, j := 0, len(p.Cells)-1; i < j; i, j = i+1, j-1 {
		p.Cells[i], p.Cells[j] = p.Cells[j], p.Cells[i]
	}
	flip := map[string]string{"┌": "└", "└": "┌", "┐": "┘", "┘": "┐", "╭": "╰", "╰": "╭", "╮": "╯", "╯": "╮",
		"┬": "┴", "┴": "┬", "▼": "▲", "▲": "▼", "▽": "△", "△": "▽"}
	for _, row := range p.Cells {
		for i := range row {
			if f, ok := flip[row[i].S]; ok {
				row[i].S = f
			}
		}
	}
}
