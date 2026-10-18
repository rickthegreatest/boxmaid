package renderer

import (
	"slices"
	"sort"
)

// The layered ("Sugiyama") layout. Work happens in two abstract axes: r runs
// along the flow (down for TB, right for LR) and o across it. Each step:
//
//  1. rank: break cycles by reversing edges, give every node a rank; edges
//     span two ranks per unit of length so labels get a rank of their own
//  2. chains: an edge across several ranks becomes a chain of dummies (and
//     its label), so every segment joins neighbouring ranks
//  3. order: sweep ranks, sorting by the neighbours' average place, keeping
//     the order with the fewest crossings
//  4. place: give each item its position across, straightening chains and
//     centring parents over children
//  5. route: in each gap between ranks, horizontal runs get tracks of their
//     own so they never overlap
//
// A subgraph is a nested level, laid out first and then placed as one block.
// An edge from outside to a node inside crosses the block's frame at a port:
// the inner level routes from the port to the node, the outer level to the
// port.

type itemKind uint8

const (
	kNode itemKind = iota
	kCluster
	kDummy
	kLabel
	kPort
)

type item struct {
	kind      itemKind
	node      *node
	sub       *level
	clusterID string
	lines     []string // the text shown: a node's wrapped label, or an edge label
	detail    []string
	w, h      int // xy footprint
	boxX      int // where the box starts within the footprint (self loops sit beside)
	boxY      int
	boxW      int
	boxH      int
	rank      int
	order     int
	pos       int // o-axis start
	r         int // r-axis start
	loops     []*edge
	edgeIdx   int  // kPort: the edge crossing here
	minO      int  // the least size across the flow (room for ports)
	top       bool // kPort: on the top (incoming) border
	ups       []*seg
	downs     []*seg
	ports     map[*seg]int // o offsets of ports that aren't central
	osize     int
	rsize     int
	labelOf   *ledge
}

type ledge struct {
	idx          int
	e            *edge
	a, b         *item // a is the upper end in layout order
	rev          bool  // reversed to break a cycle
	aIsTo        bool  // the a end is the edge's "to" end
	aReal, bReal bool  // the end is the actual node (not a frame port)
	label        []string
	chain        []*item
	length       int
}

type seg struct {
	le   *ledge
	a, b *item
}

type level struct {
	g       *graph
	cl      *cluster
	dir     string // TB, BT, LR, RL
	framed  bool
	wrap    int
	ascii   bool
	items   []*item
	direct  map[string]*item // node id or child cluster id → item
	edges   []*ledge
	ranks   map[int][]*item
	lo, hi  int // rank range (ports included)
	ports   []portReq
	portPos map[int]int
	W, H    int // xy size
	R, O    int // layout extents
	rStart  map[int]int
	rSize   map[int]int
	tracks  map[*seg]int
	gapAt   map[int]int // gap start below rank r
	gapLen  map[int]int
	pending []endMark // multiplicities to write once the lines are drawn
}

type portReq struct {
	idx int
	top bool
}

func (l *level) lr() bool { return l.dir == "LR" || l.dir == "RL" }

// layoutGraph lays out and draws g.
func layoutGraph(g *graph, dir string, wrap int, ascii bool) *Picture {
	root := &level{g: g, cl: g.root, dir: dir, wrap: wrap, ascii: ascii}
	root.process()
	cv := newCanvas(root.W+2, root.H+2, ascii)
	root.draw(cv, 0, 0)
	return cv.picture()
}

// ---- membership ----

// locate finds the item of this level that holds an edge end: the node
// itself (real) or the subgraph containing it (not real). ok is false when
// the end is outside this level.
func (l *level) locate(id string) (it *item, real, ok bool) {
	if n, isNode := l.g.nodes[id]; isNode {
		if n.cluster == l.cl {
			return l.direct[id], true, true
		}
		return l.climb(n.cluster)
	}
	if c, isCl := l.g.clusters[id]; isCl {
		if c.parent == l.cl {
			return l.direct[c.id], true, true
		}
		if c == l.cl {
			return nil, false, false
		}
		return l.climb(c.parent)
	}
	return nil, false, false
}

func (l *level) climb(c *cluster) (*item, bool, bool) {
	for cur := c; cur != nil; cur = cur.parent {
		if cur.parent == l.cl {
			return l.direct[cur.id], false, true
		}
		if cur == l.cl {
			return nil, false, false
		}
	}
	return nil, false, false
}

// ---- the whole pass ----

func (l *level) process() {
	l.direct = map[string]*item{}
	for _, id := range l.cl.nodes {
		n := l.g.nodes[id]
		if n == nil {
			continue
		}
		it := &item{kind: kNode, node: n}
		l.items = append(l.items, it)
		l.direct[id] = it
	}
	for _, c := range l.cl.children {
		it := &item{kind: kCluster, clusterID: c.id, sub: &level{g: l.g, cl: c, framed: true, wrap: l.wrap, ascii: l.ascii}}
		l.items = append(l.items, it)
		l.direct[c.id] = it
	}
	l.collectEdges()
	l.rank()
	l.addPorts()
	for _, it := range l.items {
		if it.kind == kCluster {
			l.layoutChild(it)
		}
	}
	l.reservePorts()
	for _, it := range l.items {
		l.size(it)
	}
	l.makeChains()
	l.order()
	l.assignPorts()
	l.place()
	l.route()
	l.finishSize()
}

func (l *level) collectEdges() {
	for idx, e := range l.g.edges {
		a, aReal, okA := l.locate(e.from)
		b, bReal, okB := l.locate(e.to)
		if !okA || !okB || a == nil || b == nil {
			continue
		}
		if a == b {
			if a.kind == kNode && e.from == e.to {
				a.loops = append(a.loops, e)
			}
			continue
		}
		l.edges = append(l.edges, &ledge{idx: idx, e: e, a: a, b: b, aReal: aReal, bReal: bReal, label: e.label, length: 2 * max(1, e.minlen)})
	}
}

// rank breaks cycles (reversing the edges that close them) and gives every
// item its rank by longest path, then pulls sources down next to what they
// feed.
func (l *level) rank() {
	out := map[*item][]*ledge{}
	for _, le := range l.edges {
		out[le.a] = append(out[le.a], le)
	}
	state := map[*item]int{} // 0 new, 1 on stack, 2 done
	var visit func(*item)
	visit = func(it *item) {
		state[it] = 1
		for _, le := range out[it] {
			switch state[le.b] {
			case 0:
				visit(le.b)
			case 1:
				le.rev = true
			}
		}
		state[it] = 2
	}
	// Start from items nothing points to, so the natural top stays on top.
	hasIn := map[*item]bool{}
	for _, le := range l.edges {
		hasIn[le.b] = true
	}
	for _, it := range l.items {
		if !hasIn[it] && state[it] == 0 {
			visit(it)
		}
	}
	for _, it := range l.items {
		if state[it] == 0 {
			visit(it)
		}
	}
	for _, le := range l.edges {
		if le.rev {
			le.a, le.b = le.b, le.a
			le.aReal, le.bReal = le.bReal, le.aReal
			le.aIsTo = true
		}
	}
	// Longest path over the now acyclic edges.
	indeg := map[*item]int{}
	outs := map[*item][]*ledge{}
	for _, le := range l.edges {
		indeg[le.b]++
		outs[le.a] = append(outs[le.a], le)
	}
	var queue []*item
	for _, it := range l.items {
		if indeg[it] == 0 {
			queue = append(queue, it)
		}
	}
	rank := map[*item]int{}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		for _, le := range outs[it] {
			rank[le.b] = max(rank[le.b], rank[it]+le.length)
			if indeg[le.b]--; indeg[le.b] == 0 {
				queue = append(queue, le.b)
			}
		}
	}
	// Sources sit just above the first thing they feed.
	for range 4 {
		for _, it := range l.items {
			if hasInEdge(l.edges, it) || len(outs[it]) == 0 {
				continue
			}
			best := 1 << 30
			for _, le := range outs[it] {
				best = min(best, rank[le.b]-le.length)
			}
			if best > rank[it] {
				rank[it] = best
			}
		}
	}
	lo := 0
	if len(l.items) > 0 {
		lo = 1 << 30
		for _, it := range l.items {
			lo = min(lo, rank[it])
		}
	}
	for _, it := range l.items {
		it.rank = rank[it] - lo
	}
}

func hasInEdge(edges []*ledge, it *item) bool {
	for _, le := range edges {
		if le.b == it {
			return true
		}
	}
	return false
}

// addPorts adds this level's frame ports: a top border rank above
// everything and a bottom border rank below.
func (l *level) addPorts() {
	l.lo, l.hi = 0, 0
	for _, it := range l.items {
		l.hi = max(l.hi, it.rank)
	}
	if !l.framed {
		return
	}
	top, bottom := l.lo-1, l.hi+1
	for _, p := range l.ports {
		e := l.g.edges[p.idx]
		inner, real, ok := l.locate(e.to)
		innerIsTo := true
		if !ok || inner == nil {
			inner, real, ok = l.locate(e.from)
			innerIsTo = false
		}
		if !ok || inner == nil {
			continue
		}
		port := &item{kind: kPort, edgeIdx: p.idx, top: p.top}
		l.items = append(l.items, port)
		le := &ledge{idx: p.idx, e: e}
		if p.top {
			port.rank = top
			le.a, le.b, le.aReal, le.bReal = port, inner, false, real
			le.aIsTo = !innerIsTo
		} else {
			port.rank = bottom
			le.a, le.b, le.aReal, le.bReal = inner, port, real, false
			le.aIsTo = innerIsTo
		}
		le.length = le.b.rank - le.a.rank
		l.edges = append(l.edges, le)
	}
	l.lo, l.hi = top, bottom
}

// layoutChild works out which edges cross a subgraph's frame, and where,
// then lays the subgraph out.
func (l *level) layoutChild(it *item) {
	sub := it.sub
	for _, le := range l.edges {
		switch {
		case le.a == it && !le.aReal:
			sub.ports = append(sub.ports, portReq{le.idx, false})
		case le.b == it && !le.bReal:
			sub.ports = append(sub.ports, portReq{le.idx, true})
		}
	}
	// Like Mermaid: a subgraph keeps its own direction only when nothing
	// outside links into it.
	sub.dir = l.dir
	if len(sub.ports) == 0 && sub.cl.dir != "" {
		sub.dir = sub.cl.dir
	}
	sub.process()
}

// size gives an item its footprint.
func (l *level) size(it *item) {
	switch it.kind {
	case kCluster:
		it.w, it.h = it.sub.W, it.sub.H
		it.boxW, it.boxH = it.w, it.h
	case kNode:
		l.nodeBox(it)
	case kPort:
		it.w, it.h = 1, 1
	}
	l.toAxes(it)
}

// reservePorts makes room on a node's sides for edges that will need a spot
// of their own (see assignPorts): two cells each, so a narrow box doesn't
// make them collapse onto one point.
func (l *level) reservePorts() {
	in, out := map[*item]int{}, map[*item]int{}
	marked := map[*item]bool{}
	for _, le := range l.edges {
		out[le.a]++
		in[le.b]++
		aHead, bHead := le.ends()
		if aHead != headNone || le.rev || le.e.fromLabel != "" || le.e.toLabel != "" {
			marked[le.a] = true
		}
		if bHead != headNone || le.rev || le.e.fromLabel != "" || le.e.toLabel != "" {
			marked[le.b] = true
		}
	}
	for _, it := range l.items {
		// Opposite sides can each use their centre. Only multiple edges on
		// the same side need more room; a linear chain stays three rows high.
		if it.kind == kNode && marked[it] && max(in[it], out[it]) > 1 {
			it.minO = 2*max(in[it], out[it]) + 3
		}
	}
}

func (l *level) toAxes(it *item) {
	if l.lr() {
		it.osize, it.rsize = it.h, it.w
	} else {
		it.osize, it.rsize = it.w, it.h
	}
	if it.kind == kPort || it.kind == kDummy {
		it.osize, it.rsize = 1, 0
		if it.kind == kPort {
			it.rsize = 1
		}
	}
}

func (l *level) nodeBox(it *item) {
	n := it.node
	it.lines = wrap(n.label, l.wrap)
	it.detail = wrap(n.detail, max(l.wrap, 16))
	tw := max(maxWidth(it.lines), maxWidth(it.detail), maxWidth(n.header))
	lines := len(it.lines) + len(it.detail) + len(n.header)
	if len(it.detail) > 0 {
		lines++ // a blank line between name and detail
	}
	for _, s := range n.sections {
		tw = max(tw, maxWidth(s))
		lines += 1 + max(1, len(s)) // a separator, then the compartment
	}
	w, h := tw+4, lines+2
	switch n.shape {
	case shapeRhombus, shapeHexagon:
		w = tw + 6
	case shapeCylinder:
		h++
	case shapeCircle, shapeDoubleCircle:
		w = tw + 6
	case shapeStart, shapeEnd:
		w, h = 1, 1
	case shapeBar:
		w, h = 9, 1
		if l.lr() {
			w, h = 1, 5
		}
	case shapeText:
		w, h = tw, lines
	}
	if n.shape != shapeStart && n.shape != shapeEnd && n.shape != shapeBar && n.shape != shapeText {
		w = max(w, 5)
	}
	if l.lr() {
		h = max(h, it.minO)
	} else {
		w = max(w, it.minO)
	}
	it.boxW, it.boxH = w, h
	it.w, it.h = w, h
	if len(it.loops) > 0 {
		lw := 0
		for _, e := range it.loops {
			lw = max(lw, maxWidth(e.label))
		}
		if l.lr() {
			it.boxY = 2
			it.h += 2
			it.w = max(it.w, w/2+4+lw)
		} else {
			it.h = max(it.h, 4)
			it.boxH = it.h
			it.w += 3
			if lw > 0 {
				it.w += lw + 1
			}
		}
	}
}

// makeChains splits edges into segments between neighbouring ranks, with
// dummies (and the label) in between.
func (l *level) makeChains() {
	for _, le := range l.edges {
		le.chain = []*item{le.a}
		labelAt := le.a.rank + 1
		if le.rev {
			labelAt = le.b.rank - 1
		}
		for r := le.a.rank + 1; r < le.b.rank; r++ {
			d := &item{kind: kDummy, rank: r}
			if r == labelAt && len(le.label) > 0 && le.e.style != lineInvisible {
				d.kind = kLabel
				d.labelOf = le
				d.lines = wrap(le.label, max(12, l.wrap-4))
				d.w, d.h = maxWidth(d.lines), len(d.lines)
				if l.lr() {
					d.w += 2
				}
			}
			l.toAxes(d)
			l.items = append(l.items, d)
			le.chain = append(le.chain, d)
		}
		le.chain = append(le.chain, le.b)
		for i := 0; i+1 < len(le.chain); i++ {
			s := &seg{le: le, a: le.chain[i], b: le.chain[i+1]}
			s.a.downs = append(s.a.downs, s)
			s.b.ups = append(s.b.ups, s)
		}
	}
	l.ranks = map[int][]*item{}
}

// order puts each rank in an initial order (depth first from the sources)
// and improves it by barycenter sweeps.
func (l *level) order() {
	seen := map[*item]bool{}
	var visit func(*item)
	visit = func(it *item) {
		if seen[it] {
			return
		}
		seen[it] = true
		l.ranks[it.rank] = append(l.ranks[it.rank], it)
		for _, s := range it.downs {
			visit(s.b)
		}
	}
	for _, it := range l.items {
		if it.kind == kPort && it.top {
			visit(it)
		}
	}
	for _, it := range l.items {
		if len(it.ups) == 0 {
			visit(it)
		}
	}
	for _, it := range l.items {
		visit(it)
	}
	l.renumber()
	best := l.snapshot()
	bestX := l.crossings()
	for iter := range 24 {
		if iter%2 == 0 {
			for r := l.lo + 1; r <= l.hi; r++ {
				l.sortRank(r, true)
			}
		} else {
			for r := l.hi - 1; r >= l.lo; r-- {
				l.sortRank(r, false)
			}
		}
		if x := l.crossings(); x < bestX {
			bestX, best = x, l.snapshot()
		}
		if bestX == 0 {
			break
		}
	}
	l.restore(best)
}

func (l *level) renumber() {
	for _, rk := range l.ranks {
		for i, it := range rk {
			it.order = i
		}
	}
}

func (l *level) snapshot() map[int][]*item {
	s := map[int][]*item{}
	for r, rk := range l.ranks {
		s[r] = slices.Clone(rk)
	}
	return s
}

func (l *level) restore(s map[int][]*item) {
	l.ranks = s
	l.renumber()
}

func (l *level) sortRank(r int, up bool) {
	rk := l.ranks[r]
	key := map[*item]float64{}
	for _, it := range rk {
		segs := it.downs
		if up {
			segs = it.ups
		}
		if len(segs) == 0 {
			key[it] = float64(it.order)
			continue
		}
		sum := 0.0
		for _, s := range segs {
			if up {
				sum += float64(s.a.order)
			} else {
				sum += float64(s.b.order)
			}
		}
		key[it] = sum / float64(len(segs))
	}
	sort.SliceStable(rk, func(i, j int) bool { return key[rk[i]] < key[rk[j]] })
	for i, it := range rk {
		it.order = i
	}
}

func (l *level) crossings() int {
	n := 0
	for r := l.lo; r < l.hi; r++ {
		var segs []*seg
		for _, it := range l.ranks[r] {
			segs = append(segs, it.downs...)
		}
		for i := range segs {
			for j := i + 1; j < len(segs); j++ {
				a, b := segs[i], segs[j]
				if (a.a.order-b.a.order)*(a.b.order-b.b.order) < 0 {
					n++
				}
			}
		}
	}
	return n
}

// assignPorts gives edges that would collide at a node their own spot on
// its border: reversed edges (going back up) move off the centre, so they
// don't merge with the edges flowing down.
func (l *level) assignPorts() {
	for _, it := range l.items {
		if it.kind != kNode {
			continue
		}
		it.ports = map[*seg]int{}
		for _, side := range [][]*seg{it.ups, it.downs} {
			// Segments that need a spot of their own: edges going back up
			// (so they don't merge with the flow) and, when several edges
			// end here with a marker (◆ ◇ △), each of those.
			headed := func(s *seg) bool { // a marker or a multiplicity at this end
				aHead, bHead := s.le.ends()
				aLabel, bLabel := s.le.e.fromLabel, s.le.e.toLabel
				if s.le.aIsTo {
					aLabel, bLabel = bLabel, aLabel
				}
				return s.le.a == it && (aHead != headNone || aLabel != "") || s.le.b == it && (bHead != headNone || bLabel != "")
			}
			nHeaded := 0
			for _, s := range side {
				if headed(s) {
					nHeaded++
				}
			}
			var own []*seg
			for _, s := range side {
				// A marked edge shares no spot with others (its marker would
				// seem to belong to them too).
				if s.le.rev || headed(s) && len(side) > 1 {
					own = append(own, s)
				}
			}
			if len(own) == 0 {
				continue
			}
			// Spread them evenly, in the order of their other ends, so they
			// don't cross on the way out.
			sort.SliceStable(own, func(i, j int) bool { return other(own[i], it).order < other(own[j], it).order })
			lo, hi := it.boxLo(l)+1, it.boxLo(l)+it.boxO(l)-2
			c := it.boxLo(l) + it.boxO(l)/2
			shared := len(own) < len(side)
			step := max(2, (hi-lo+1)/(len(own)+1))
			for k, s := range own {
				p := lo - 1 + step*(k+1)
				if shared && p == c {
					p++ // leave the centre to the edges that share it
				}
				it.ports[s] = max(lo, min(hi, p))
			}
		}
	}
}

// other is the item at a segment's far end from it.
func other(s *seg, it *item) *item {
	if s.a == it {
		return s.b
	}
	return s.a
}

// boxLo and boxO are where the drawn box starts across, and its size.
func (it *item) boxLo(l *level) int {
	if l.lr() {
		return it.boxY
	}
	return it.boxX
}

func (it *item) boxO(l *level) int {
	if it.boxW == 0 {
		return it.osize
	}
	if l.lr() {
		return it.boxH
	}
	return it.boxW
}

// portOff is where along the item (across the flow) a segment attaches.
func (l *level) portOff(it *item, s *seg) int {
	switch it.kind {
	case kNode:
		if o, ok := it.ports[s]; ok {
			return o
		}
		return it.boxLo(l) + it.boxO(l)/2
	case kCluster:
		real := (s.le.a == it && s.le.aReal) || (s.le.b == it && s.le.bReal)
		if !real {
			if o, ok := it.sub.portPos[s.le.idx]; ok {
				return o
			}
		}
		return it.osize / 2
	case kLabel:
		return it.osize / 2
	}
	return 0
}

func (l *level) sep(a, b *item) int {
	big := func(it *item) bool { return it.kind == kNode || it.kind == kCluster }
	if l.lr() {
		return 1
	}
	if big(a) && big(b) {
		return 3
	}
	if a.kind == kDummy && b.kind == kDummy {
		return 1
	}
	return 2
}

// place assigns positions across the flow: pack, then pull each item
// towards its neighbours, most important first (long edges stay straight).
func (l *level) place() {
	for r := l.lo; r <= l.hi; r++ {
		x := 0
		for _, it := range l.ranks[r] {
			it.pos = x
			x += it.osize + l.sep(it, it)
		}
	}
	for range 6 {
		for r := l.lo + 1; r <= l.hi; r++ {
			l.placeRank(r, true)
		}
		for r := l.hi - 1; r >= l.lo; r-- {
			l.placeRank(r, false)
		}
	}
	// Normalise: nothing left of zero, then leave room for the frame.
	minPos := 1 << 30
	for _, rk := range l.ranks {
		for _, it := range rk {
			minPos = min(minPos, it.pos)
		}
	}
	margin := 0
	if l.framed {
		margin = 2
		if l.lr() {
			margin = 1
		}
	}
	for _, rk := range l.ranks {
		for _, it := range rk {
			it.pos += margin - minPos
		}
	}
}

func (l *level) placeRank(r int, up bool) {
	rk := l.ranks[r]
	if len(rk) == 0 {
		return
	}
	desired := make([]int, len(rk))
	prio := make([]int, len(rk))
	for i, it := range rk {
		segs := it.downs
		if up {
			segs = it.ups
		}
		desired[i] = it.pos
		if len(segs) > 0 {
			lo, hi := 1<<30, -(1 << 30)
			for _, s := range segs {
				var other *item
				if up {
					other = s.a
				} else {
					other = s.b
				}
				p := other.pos + l.portOff(other, s) - l.portOff(it, s)
				lo, hi = min(lo, p), max(hi, p)
			}
			desired[i] = (lo + hi) / 2
		}
		switch it.kind {
		case kDummy, kLabel, kPort:
			prio[i] = 1000 + len(segs)
		default:
			prio[i] = len(it.ups) + len(it.downs)
		}
	}
	idx := make([]int, len(rk))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return prio[idx[a]] > prio[idx[b]] })
	placed := make([]bool, len(rk))
	for _, i := range idx {
		lo, hi := -(1 << 30), 1<<30
		// Room needed between i and each placed neighbour.
		need := 0
		for j := i - 1; j >= 0; j-- {
			need += rk[j].osize + l.sep(rk[j], rk[j+1])
			if placed[j] {
				lo = max(lo, rk[j].pos+need)
				break
			}
		}
		need = 0
		for j := i + 1; j < len(rk); j++ {
			need += rk[j-1].osize + l.sep(rk[j-1], rk[j])
			if j-1 == i {
				need = rk[i].osize + l.sep(rk[i], rk[j])
			}
			if placed[j] {
				hi = min(hi, rk[j].pos-need)
				break
			}
		}
		p := desired[i]
		if p < lo {
			p = lo
		}
		if p > hi && hi >= lo {
			p = hi
		}
		rk[i].pos = p
		placed[i] = true
	}
	for i := 1; i < len(rk); i++ {
		if min := rk[i-1].pos + rk[i-1].osize + l.sep(rk[i-1], rk[i]); rk[i].pos < min {
			rk[i].pos = min
		}
	}
}

// ---- routing ----

// net is the horizontal part of the segments leaving one port into a gap.
type net struct {
	segs   []*seg
	src    int
	drops  []int
	lo, hi int
	track  int
}

func (l *level) stubs() (a, b, straight int) {
	if l.lr() {
		return 2, 2, 2
	}
	return 1, 1, 1
}

func (l *level) route() {
	l.rSize = map[int]int{}
	for r := l.lo; r <= l.hi; r++ {
		for _, it := range l.ranks[r] {
			l.rSize[r] = max(l.rSize[r], it.rsize)
		}
	}
	l.tracks = map[*seg]int{}
	l.gapLen = map[int]int{}
	stubA, stubB, straight := l.stubs()
	for r := l.lo; r < l.hi; r++ {
		nets := map[[2]int]*net{}
		var order []*net
		any := false
		for _, it := range l.ranks[r] {
			for _, s := range it.downs {
				any = true
				x1 := it.pos + l.portOff(it, s)
				key := [2]int{it.order, x1}
				n := nets[key]
				if n == nil {
					n = &net{src: x1, lo: x1, hi: x1}
					nets[key] = n
					order = append(order, n)
				}
				x2 := s.b.pos + l.portOff(s.b, s)
				n.segs = append(n.segs, s)
				n.drops = append(n.drops, x2)
				n.lo, n.hi = min(n.lo, x2), max(n.hi, x2)
			}
		}
		var bent []*net
		for _, n := range order {
			if n.lo != n.hi {
				bent = append(bent, n)
			}
		}
		t := assignTracks(bent)
		for _, n := range bent {
			for _, s := range n.segs {
				l.tracks[s] = n.track
			}
		}
		switch {
		case t > 0:
			l.gapLen[r] = stubA + t + stubB
		case any:
			l.gapLen[r] = straight
		default:
			l.gapLen[r] = 1
		}
	}
	// Rank coordinates.
	l.rStart = map[int]int{}
	l.gapAt = map[int]int{}
	y := 0
	for r := l.lo; r <= l.hi; r++ {
		l.rStart[r] = y
		y += l.rSize[r]
		l.gapAt[r] = y
		if r < l.hi {
			y += l.gapLen[r]
		}
	}
	l.R = y
	for r := l.lo; r <= l.hi; r++ {
		for _, it := range l.ranks[r] {
			it.r = l.rStart[r] + (l.rSize[r]-it.rsize)/2
		}
	}
}

// assignTracks gives each bent net a track (0 = nearest the upper rank) so
// that nets on one track don't overlap, and a net whose riser stands where
// another net drops goes above it (so the two never share a line).
func assignTracks(nets []*net) int {
	if len(nets) == 0 {
		return 0
	}
	above := map[*net][]*net{} // n → nets that must be above n
	for _, a := range nets {
		for _, b := range nets {
			if a != b && slices.Contains(b.drops, a.src) {
				above[b] = append(above[b], a)
			}
		}
	}
	sort.SliceStable(nets, func(i, j int) bool {
		if nets[i].lo != nets[j].lo {
			return nets[i].lo < nets[j].lo
		}
		return nets[i].hi-nets[i].lo > nets[j].hi-nets[j].lo
	})
	done := map[*net]bool{}
	track := map[*net]int{}
	var byTrack [][]*net
	assigned := 0
	for assigned < len(nets) {
		progress := false
		for _, n := range nets {
			if done[n] {
				continue
			}
			ready := true
			minT := 0
			for _, a := range above[n] {
				if !done[a] {
					ready = false
					break
				}
				minT = max(minT, track[a]+1)
			}
			if !ready {
				continue
			}
			t := minT
			for ; t < len(byTrack); t++ {
				free := true
				for _, o := range byTrack[t] {
					if n.lo <= o.hi+1 && o.lo <= n.hi+1 {
						free = false
						break
					}
				}
				if free {
					break
				}
			}
			for t >= len(byTrack) {
				byTrack = append(byTrack, nil)
			}
			byTrack[t] = append(byTrack[t], n)
			track[n], n.track = t, t
			done[n] = true
			assigned++
			progress = true
		}
		if !progress {
			// A cycle of constraints: drop them for the rest.
			for _, n := range nets {
				if !done[n] {
					above[n] = nil
				}
			}
		}
	}
	return len(byTrack)
}

// finishSize works out the level's size, making room for a frame's title.
func (l *level) finishSize() {
	l.O = 0
	for _, rk := range l.ranks {
		for _, it := range rk {
			l.O = max(l.O, it.pos+it.osize)
		}
	}
	if l.framed {
		if l.lr() {
			l.O++
		} else {
			l.O += 2
		}
		title := width(l.cl.title) + 6
		if l.lr() {
			if d := title - l.R; d > 0 {
				// Widen by moving the far border out.
				l.rStart[l.hi] += d
				for _, it := range l.ranks[l.hi] {
					it.r += d
				}
				l.R = title
			}
		} else if l.O < title {
			l.O = title
		}
	}
	l.portPos = map[int]int{}
	for _, it := range l.items {
		if it.kind == kPort {
			l.portPos[it.edgeIdx] = it.pos
		}
	}
	if l.lr() {
		l.W, l.H = l.R, l.O
	} else {
		l.W, l.H = l.O, l.R
	}
}
