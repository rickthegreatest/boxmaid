package renderer

// graph is what flowcharts, state diagrams and C4 diagrams parse into, and
// what the layered layout draws.
type graph struct {
	dir      string // TB, BT, LR or RL
	nodes    map[string]*node
	order    []string // node ids in the order they first appeared
	edges    []*edge
	root     *cluster
	clusters map[string]*cluster
	edgeIDs  map[string]bool // ids given to edges ("A e1@--> B")
}

type shape uint8

const (
	shapeRect shape = iota
	shapeRound
	shapeStadium
	shapeSubroutine
	shapeCylinder
	shapeCircle
	shapeDoubleCircle
	shapeAsym
	shapeRhombus
	shapeHexagon
	shapeLeanRight // parallelogram
	shapeLeanLeft
	shapeTrapezoid
	shapeTrapezoidAlt
	shapeStart // state diagram start: a filled dot
	shapeEnd   // state diagram end: a ringed dot
	shapeBar   // state diagram fork/join
	shapeNote
	shapeText // no border
)

type node struct {
	id     string
	label  []string // the label's lines, before wrapping
	detail []string // secondary lines shown muted (C4 technology and description)
	// header lines go above the label, muted («interface», «Requirement»).
	header []string
	// sections are compartments below the label, left-aligned and
	// separated by lines: a class's attributes and methods, an entity's
	// attributes.
	sections [][]string
	shape    shape
	color    string // "#rrggbb" from classDef/style, or ""
	dashed   bool   // a dashed border (external systems)
	classes  []string
	defined  bool // the source gave it a label or shape (not just mentioned)
	cluster  *cluster
}

type lineStyle uint8

const (
	lineSolid lineStyle = iota
	lineDotted
	lineThick
	lineInvisible
)

type head uint8

const (
	headNone head = iota
	headArrow
	headCircle
	headCross
	headTriangle    // UML inheritance and realisation: a hollow triangle
	headDiamond     // composition: a filled diamond
	headDiamondOpen // aggregation: a hollow diamond
)

type edge struct {
	// fromLabel and toLabel are written next to each end (multiplicities).
	fromLabel, toLabel string
	from, to           string
	label              []string
	style              lineStyle
	head               head // at to
	tail               head // at from
	minlen             int
}

type cluster struct {
	id, title string
	dir       string // its own direction, or "" to follow the parent
	parent    *cluster
	children  []*cluster
	nodes     []string
}

func newGraph(dir string) *graph {
	root := &cluster{}
	return &graph{dir: dir, nodes: map[string]*node{}, root: root, clusters: map[string]*cluster{}, edgeIDs: map[string]bool{}}
}

// node returns the node with id, creating it in c when it's new. A node
// first mentioned at the top level moves into the first subgraph that
// mentions it, like Mermaid does.
func (g *graph) node(id string, c *cluster) *node {
	n, ok := g.nodes[id]
	if !ok {
		n = &node{id: id, label: []string{id}, cluster: c}
		g.nodes[id] = n
		g.order = append(g.order, id)
		c.nodes = append(c.nodes, id)
		return n
	}
	if n.cluster == g.root && c != g.root {
		g.moveNode(id, c)
	}
	return n
}

// moveNode keeps both sides of membership in sync, including redeclarations.
func (g *graph) moveNode(id string, c *cluster) {
	n := g.nodes[id]
	if n.cluster == c {
		return
	}
	n.cluster.nodes = remove(n.cluster.nodes, id)
	n.cluster = c
	c.nodes = append(c.nodes, id)
}

func (g *graph) addCluster(id, title string, parent *cluster) *cluster {
	c := &cluster{id: id, title: title, parent: parent}
	parent.children = append(parent.children, c)
	if id != "" {
		g.clusters[id] = c
	}
	return c
}

// finish resolves what can only be known at the end: an edge to a
// subgraph's id is an edge to the subgraph, not to a node of that name.
func (g *graph) finish() {
	for id := range g.clusters {
		if n, ok := g.nodes[id]; ok && !n.defined {
			delete(g.nodes, id)
			g.order = remove(g.order, id)
			n.cluster.nodes = remove(n.cluster.nodes, id)
		}
	}
	for _, e := range g.edges {
		if e.minlen < 1 {
			e.minlen = 1
		}
	}
	// Anonymous subgraphs get an id of their own, in tree order.
	n := 0
	var name func(c *cluster)
	name = func(c *cluster) {
		for _, ch := range c.children {
			if ch.id == "" {
				n++
				ch.id = "\x00cluster" + itoa(n)
				g.clusters[ch.id] = ch
			}
			name(ch)
		}
	}
	name(g.root)
}

func remove(ids []string, id string) []string {
	out := ids[:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}
