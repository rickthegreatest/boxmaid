//! The layered ("Sugiyama") layout. Work happens in two abstract axes: r
//! runs along the flow (down for TB, right for LR) and o across it. Each
//! step:
//!
//!  1. rank: break cycles by reversing edges, give every node a rank; edges
//!     span two ranks per unit of length so labels get a rank of their own
//!  2. chains: an edge across several ranks becomes a chain of dummies (and
//!     its label), so every segment joins neighbouring ranks
//!  3. order: sweep ranks, sorting by the neighbours' average place, keeping
//!     the order with the fewest crossings
//!  4. place: give each item its position across, straightening chains and
//!     centring parents over children
//!  5. route: in each gap between ranks, horizontal runs get tracks of their
//!     own so they never overlap
//!
//! A subgraph is a nested level, laid out first and then placed as one
//! block. An edge from outside to a node inside crosses the block's frame at
//! a port: the inner level routes from the port to the node, the outer level
//! to the port.
//!
//! Items, level edges and segments live in arenas and refer to each other by
//! index.

use std::collections::HashMap;

pub(crate) use crate::draw::fit_graph;
use crate::draw::EndMark;
use crate::graph::{Edge, Graph, Head, LineStyle, Shape};
use crate::text::{max_width, wrap};

const INF: i64 = 1 << 30;

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum Kind {
    Node,
    Cluster,
    Dummy,
    Label,
    Port,
}

pub(crate) struct Item<'g> {
    pub kind: Kind,
    pub node: Option<String>,
    pub sub: Option<Box<Level<'g>>>,
    /// The text shown: a node's wrapped label, or an edge label.
    pub lines: Vec<String>,
    pub detail: Vec<String>,
    /// xy footprint.
    pub w: i64,
    pub h: i64,
    /// Where the box starts within the footprint (self loops sit beside).
    pub box_x: i64,
    pub box_y: i64,
    pub box_w: i64,
    pub box_h: i64,
    pub rank: i64,
    pub order: i64,
    /// o-axis start.
    pub pos: i64,
    /// r-axis start.
    pub r: i64,
    /// Self loops, as indices into the graph's edges.
    pub loops: Vec<usize>,
    /// Port: the edge crossing here.
    pub edge_idx: usize,
    /// The least size across the flow (room for ports).
    pub min_o: i64,
    /// Port: on the top (incoming) border.
    pub top: bool,
    pub ups: Vec<usize>,
    pub downs: Vec<usize>,
    /// o offsets of ports that aren't central, by segment.
    pub ports: HashMap<usize, i64>,
    pub osize: i64,
    pub rsize: i64,
}

impl<'g> Item<'g> {
    fn new(kind: Kind) -> Item<'g> {
        Item {
            kind,
            node: None,
            sub: None,
            lines: vec![],
            detail: vec![],
            w: 0,
            h: 0,
            box_x: 0,
            box_y: 0,
            box_w: 0,
            box_h: 0,
            rank: 0,
            order: 0,
            pos: 0,
            r: 0,
            loops: vec![],
            edge_idx: 0,
            min_o: 0,
            top: false,
            ups: vec![],
            downs: vec![],
            ports: HashMap::new(),
            osize: 0,
            rsize: 0,
        }
    }
}

#[derive(Clone, Default)]
pub(crate) struct LEdge {
    pub idx: usize,
    /// The upper end in layout order.
    pub a: usize,
    pub b: usize,
    /// Reversed to break a cycle.
    pub rev: bool,
    /// The a end is the edge's "to" end.
    pub a_is_to: bool,
    /// The end is the actual node (not a frame port).
    pub a_real: bool,
    pub b_real: bool,
    pub label: Vec<String>,
    pub chain: Vec<usize>,
    pub length: i64,
}

pub(crate) struct Seg {
    pub le: usize,
    pub a: usize,
    pub b: usize,
}

pub(crate) struct Level<'g> {
    pub g: &'g Graph,
    pub cl: usize,
    /// TB, BT, LR, RL.
    pub dir: String,
    pub framed: bool,
    pub wrap: i64,
    pub ascii: bool,
    pub items: Vec<Item<'g>>,
    /// Node id or child cluster id → item.
    pub direct: HashMap<String, usize>,
    pub edges: Vec<LEdge>,
    pub segs: Vec<Seg>,
    pub ranks: HashMap<i64, Vec<usize>>,
    /// Rank range (ports included).
    pub lo: i64,
    pub hi: i64,
    pub ports: Vec<(usize, bool)>,
    pub port_pos: HashMap<usize, i64>,
    /// xy size.
    pub w: i64,
    pub h: i64,
    /// Layout extents.
    pub rr: i64,
    pub oo: i64,
    pub r_start: HashMap<i64, i64>,
    pub r_size: HashMap<i64, i64>,
    pub tracks: HashMap<usize, i64>,
    /// Gap start below rank r.
    pub gap_at: HashMap<i64, i64>,
    pub gap_len: HashMap<i64, i64>,
    /// Multiplicities to write once the lines are drawn.
    pub pending: Vec<EndMark>,
}

/// The horizontal part of the segments leaving one port into a gap.
struct Net {
    segs: Vec<usize>,
    src: i64,
    drops: Vec<i64>,
    lo: i64,
    hi: i64,
    track: i64,
}

impl<'g> Level<'g> {
    pub fn new(g: &'g Graph, cl: usize, dir: &str, framed: bool, wrap: i64, ascii: bool) -> Level<'g> {
        Level {
            g,
            cl,
            dir: dir.to_string(),
            framed,
            wrap,
            ascii,
            items: vec![],
            direct: HashMap::new(),
            edges: vec![],
            segs: vec![],
            ranks: HashMap::new(),
            lo: 0,
            hi: 0,
            ports: vec![],
            port_pos: HashMap::new(),
            w: 0,
            h: 0,
            rr: 0,
            oo: 0,
            r_start: HashMap::new(),
            r_size: HashMap::new(),
            tracks: HashMap::new(),
            gap_at: HashMap::new(),
            gap_len: HashMap::new(),
            pending: vec![],
        }
    }

    pub fn lr(&self) -> bool {
        self.dir == "LR" || self.dir == "RL"
    }

    pub fn edge(&self, le: &LEdge) -> &'g Edge {
        &self.g.edges[le.idx]
    }

    pub fn rank_items(&self, r: i64) -> &[usize] {
        self.ranks.get(&r).map_or(&[], |v| v.as_slice())
    }

    // ---- membership ----

    /// Finds the item of this level that holds an edge end: the node itself
    /// (real) or the subgraph containing it (not real). ok is false when the
    /// end is outside this level.
    fn locate(&self, id: &str) -> (Option<usize>, bool, bool) {
        if let Some(n) = self.g.nodes.get(id) {
            if n.cluster == self.cl {
                return (self.direct.get(id).copied(), true, true);
            }
            return self.climb(n.cluster);
        }
        if let Some(&c) = self.g.clusters.get(id) {
            if self.g.cl[c].parent == Some(self.cl) {
                return (self.direct.get(&self.g.cl[c].id).copied(), true, true);
            }
            if c == self.cl {
                return (None, false, false);
            }
            return match self.g.cl[c].parent {
                Some(p) => self.climb(p),
                None => (None, false, false),
            };
        }
        (None, false, false)
    }

    fn climb(&self, c: usize) -> (Option<usize>, bool, bool) {
        let mut cur = Some(c);
        while let Some(k) = cur {
            if self.g.cl[k].parent == Some(self.cl) {
                return (self.direct.get(&self.g.cl[k].id).copied(), false, true);
            }
            if k == self.cl {
                return (None, false, false);
            }
            cur = self.g.cl[k].parent;
        }
        (None, false, false)
    }

    // ---- the whole pass ----

    pub fn process(&mut self) {
        let g = self.g;
        for id in &g.cl[self.cl].nodes {
            if !g.nodes.contains_key(id) {
                continue;
            }
            let mut it = Item::new(Kind::Node);
            it.node = Some(id.clone());
            self.items.push(it);
            self.direct.insert(id.clone(), self.items.len() - 1);
        }
        for &c in &g.cl[self.cl].children {
            let mut it = Item::new(Kind::Cluster);
            it.sub = Some(Box::new(Level::new(g, c, "", true, self.wrap, self.ascii)));
            self.items.push(it);
            self.direct.insert(g.cl[c].id.clone(), self.items.len() - 1);
        }
        self.collect_edges();
        self.rank();
        self.add_ports();
        for i in 0..self.items.len() {
            if self.items[i].kind == Kind::Cluster {
                self.layout_child(i);
            }
        }
        self.reserve_ports();
        for i in 0..self.items.len() {
            self.size(i);
        }
        self.make_chains();
        self.order();
        self.assign_ports();
        self.place();
        self.route();
        self.finish_size();
    }

    fn collect_edges(&mut self) {
        for (idx, e) in self.g.edges.iter().enumerate() {
            let (a, a_real, ok_a) = self.locate(&e.from);
            let (b, b_real, ok_b) = self.locate(&e.to);
            let (Some(a), Some(b)) = (a, b) else { continue };
            if !ok_a || !ok_b {
                continue;
            }
            if a == b {
                if self.items[a].kind == Kind::Node && e.from == e.to {
                    self.items[a].loops.push(idx);
                }
                continue;
            }
            self.edges.push(LEdge { idx, a, b, a_real, b_real, label: e.label.clone(), length: 2 * e.minlen.max(1), ..Default::default() });
        }
    }

    /// Breaks cycles (reversing the edges that close them) and gives every
    /// item its rank by longest path, then pulls sources down next to what
    /// they feed.
    fn rank(&mut self) {
        let n = self.items.len();
        let mut out: Vec<Vec<usize>> = vec![vec![]; n];
        for (i, le) in self.edges.iter().enumerate() {
            out[le.a].push(i);
        }
        // 0 new, 1 on stack, 2 done
        let mut state = vec![0u8; n];
        fn visit(it: usize, out: &[Vec<usize>], edges: &mut [LEdge], state: &mut [u8]) {
            state[it] = 1;
            for &i in &out[it] {
                let b = edges[i].b;
                match state[b] {
                    0 => visit(b, out, edges, state),
                    1 => edges[i].rev = true,
                    _ => {}
                }
            }
            state[it] = 2;
        }
        // Start from items nothing points to, so the natural top stays on top.
        let mut has_in = vec![false; n];
        for le in &self.edges {
            has_in[le.b] = true;
        }
        for it in 0..n {
            if !has_in[it] && state[it] == 0 {
                visit(it, &out, &mut self.edges, &mut state);
            }
        }
        for it in 0..n {
            if state[it] == 0 {
                visit(it, &out, &mut self.edges, &mut state);
            }
        }
        for le in &mut self.edges {
            if le.rev {
                std::mem::swap(&mut le.a, &mut le.b);
                std::mem::swap(&mut le.a_real, &mut le.b_real);
                le.a_is_to = true;
            }
        }
        // Longest path over the now acyclic edges.
        let mut indeg = vec![0i64; n];
        let mut outs: Vec<Vec<usize>> = vec![vec![]; n];
        for (i, le) in self.edges.iter().enumerate() {
            indeg[le.b] += 1;
            outs[le.a].push(i);
        }
        let mut queue: std::collections::VecDeque<usize> = (0..n).filter(|&it| indeg[it] == 0).collect();
        let mut rank = vec![0i64; n];
        while let Some(it) = queue.pop_front() {
            for &i in &outs[it] {
                let le = &self.edges[i];
                rank[le.b] = rank[le.b].max(rank[it] + le.length);
                indeg[le.b] -= 1;
                if indeg[le.b] == 0 {
                    queue.push_back(le.b);
                }
            }
        }
        // Sources sit just above the first thing they feed.
        let mut has_in_edge = vec![false; n];
        for le in &self.edges {
            has_in_edge[le.b] = true;
        }
        for _ in 0..4 {
            for it in 0..n {
                if has_in_edge[it] || outs[it].is_empty() {
                    continue;
                }
                let mut best = INF;
                for &i in &outs[it] {
                    let le = &self.edges[i];
                    best = best.min(rank[le.b] - le.length);
                }
                if best > rank[it] {
                    rank[it] = best;
                }
            }
        }
        let lo = if n > 0 { rank.iter().copied().min().unwrap() } else { 0 };
        for (it, item) in self.items.iter_mut().enumerate() {
            item.rank = rank[it] - lo;
        }
    }

    /// Adds this level's frame ports: a top border rank above everything
    /// and a bottom border rank below.
    fn add_ports(&mut self) {
        self.lo = 0;
        self.hi = 0;
        for it in &self.items {
            self.hi = self.hi.max(it.rank);
        }
        if !self.framed {
            return;
        }
        let (top, bottom) = (self.lo - 1, self.hi + 1);
        for (idx, is_top) in self.ports.clone() {
            let e = &self.g.edges[idx];
            let (mut inner, mut real, mut ok) = self.locate(&e.to);
            let mut inner_is_to = true;
            if !ok || inner.is_none() {
                (inner, real, ok) = self.locate(&e.from);
                inner_is_to = false;
            }
            let Some(inner) = inner else { continue };
            if !ok {
                continue;
            }
            let mut port = Item::new(Kind::Port);
            port.edge_idx = idx;
            port.top = is_top;
            let mut le = LEdge { idx, ..Default::default() };
            let pi = self.items.len();
            if is_top {
                port.rank = top;
                (le.a, le.b, le.a_real, le.b_real) = (pi, inner, false, real);
                le.a_is_to = !inner_is_to;
            } else {
                port.rank = bottom;
                (le.a, le.b, le.a_real, le.b_real) = (inner, pi, real, false);
                le.a_is_to = inner_is_to;
            }
            self.items.push(port);
            le.length = self.items[le.b].rank - self.items[le.a].rank;
            self.edges.push(le);
        }
        self.lo = top;
        self.hi = bottom;
    }

    /// Works out which edges cross a subgraph's frame, and where, then lays
    /// the subgraph out.
    fn layout_child(&mut self, it: usize) {
        let mut ports = vec![];
        for le in &self.edges {
            if le.a == it && !le.a_real {
                ports.push((le.idx, false));
            } else if le.b == it && !le.b_real {
                ports.push((le.idx, true));
            }
        }
        let dir = self.dir.clone();
        let sub = self.items[it].sub.as_mut().unwrap();
        sub.ports.extend(ports);
        // Like Mermaid: a subgraph keeps its own direction only when nothing
        // outside links into it.
        sub.dir = dir;
        let g = sub.g;
        let own = &g.cl[sub.cl].dir;
        if sub.ports.is_empty() && !own.is_empty() {
            sub.dir = own.clone();
        }
        sub.process();
    }

    /// Gives an item its footprint.
    fn size(&mut self, i: usize) {
        match self.items[i].kind {
            Kind::Cluster => {
                let it = &mut self.items[i];
                let sub = it.sub.as_ref().unwrap();
                (it.w, it.h) = (sub.w, sub.h);
                (it.box_w, it.box_h) = (it.w, it.h);
            }
            Kind::Node => self.node_box(i),
            Kind::Port => {
                let it = &mut self.items[i];
                (it.w, it.h) = (1, 1);
            }
            _ => {}
        }
        let lr = self.lr();
        to_axes(lr, &mut self.items[i]);
    }

    /// Makes room on a node's sides for edges that will need a spot of
    /// their own (see assign_ports): two cells each, so a narrow box doesn't
    /// make them collapse onto one point.
    fn reserve_ports(&mut self) {
        let n = self.items.len();
        let (mut inn, mut out) = (vec![0i64; n], vec![0i64; n]);
        let mut marked = vec![false; n];
        for le in &self.edges {
            out[le.a] += 1;
            inn[le.b] += 1;
            let (a_head, b_head) = self.ends(le);
            let e = self.edge(le);
            let labelled = le.rev || !e.from_label.is_empty() || !e.to_label.is_empty();
            if a_head != Head::None || labelled {
                marked[le.a] = true;
            }
            if b_head != Head::None || labelled {
                marked[le.b] = true;
            }
        }
        for (i, it) in self.items.iter_mut().enumerate() {
            // A single edge has the centre to itself.
            if it.kind == Kind::Node && marked[i] && inn[i] + out[i] > 1 {
                it.min_o = 2 * inn[i].max(out[i]) + 3;
            }
        }
    }

    fn node_box(&mut self, i: usize) {
        let lr = self.lr();
        let wrap_w = self.wrap;
        let g = self.g;
        let it = &mut self.items[i];
        let n = &g.nodes[it.node.as_ref().unwrap()];
        it.lines = wrap(&n.label, wrap_w);
        it.detail = wrap(&n.detail, wrap_w.max(16));
        let mut tw = max_width(&it.lines).max(max_width(&it.detail)).max(max_width(&n.header));
        let mut lines = (it.lines.len() + it.detail.len() + n.header.len()) as i64;
        if !it.detail.is_empty() {
            lines += 1; // a blank line between name and detail
        }
        for s in &n.sections {
            tw = tw.max(max_width(s));
            lines += 1 + (s.len() as i64).max(1); // a separator, then the compartment
        }
        let (mut w, mut h) = (tw + 4, lines + 2);
        match n.shape {
            Shape::Rhombus | Shape::Hexagon => w = tw + 6,
            Shape::Cylinder => h += 1,
            Shape::Circle | Shape::DoubleCircle => w = tw + 6,
            Shape::Start | Shape::End => (w, h) = (1, 1),
            Shape::Bar => {
                (w, h) = (9, 1);
                if lr {
                    (w, h) = (1, 5);
                }
            }
            Shape::Text => (w, h) = (tw, lines),
            _ => {}
        }
        if !matches!(n.shape, Shape::Start | Shape::End | Shape::Bar | Shape::Text) {
            w = w.max(5);
        }
        if lr {
            h = h.max(it.min_o);
        } else {
            w = w.max(it.min_o);
        }
        (it.box_w, it.box_h) = (w, h);
        (it.w, it.h) = (w, h);
        if !it.loops.is_empty() {
            let mut lw = 0;
            for &e in &it.loops {
                lw = lw.max(max_width(&g.edges[e].label));
            }
            if lr {
                it.box_y = 2;
                it.h += 2;
                it.w = it.w.max(w / 2 + 4 + lw);
            } else {
                it.h = it.h.max(4);
                it.box_h = it.h;
                it.w += 3;
                if lw > 0 {
                    it.w += lw + 1;
                }
            }
        }
    }

    /// Splits edges into segments between neighbouring ranks, with dummies
    /// (and the label) in between.
    fn make_chains(&mut self) {
        let lr = self.lr();
        for li in 0..self.edges.len() {
            let (a, b, rev) = (self.edges[li].a, self.edges[li].b, self.edges[li].rev);
            let (ar, br) = (self.items[a].rank, self.items[b].rank);
            let mut chain = vec![a];
            let label_at = if rev { br - 1 } else { ar + 1 };
            let visible = self.edge(&self.edges[li]).style != LineStyle::Invisible;
            for r in ar + 1..br {
                let mut d = Item::new(Kind::Dummy);
                d.rank = r;
                if r == label_at && !self.edges[li].label.is_empty() && visible {
                    d.kind = Kind::Label;
                    d.lines = wrap(&self.edges[li].label, 12.max(self.wrap - 4));
                    (d.w, d.h) = (max_width(&d.lines), d.lines.len() as i64);
                    if lr {
                        d.w += 2;
                    }
                }
                to_axes(lr, &mut d);
                self.items.push(d);
                chain.push(self.items.len() - 1);
            }
            chain.push(b);
            for k in 0..chain.len() - 1 {
                let si = self.segs.len();
                self.segs.push(Seg { le: li, a: chain[k], b: chain[k + 1] });
                self.items[chain[k]].downs.push(si);
                self.items[chain[k + 1]].ups.push(si);
            }
            self.edges[li].chain = chain;
        }
        self.ranks = HashMap::new();
    }

    /// Puts each rank in an initial order (depth first from the sources) and
    /// improves it by barycenter sweeps.
    fn order(&mut self) {
        let mut seen = vec![false; self.items.len()];
        fn visit(l: &mut Level, it: usize, seen: &mut [bool]) {
            if seen[it] {
                return;
            }
            seen[it] = true;
            let r = l.items[it].rank;
            l.ranks.entry(r).or_default().push(it);
            for k in 0..l.items[it].downs.len() {
                let b = l.segs[l.items[it].downs[k]].b;
                visit(l, b, seen);
            }
        }
        for it in 0..self.items.len() {
            if self.items[it].kind == Kind::Port && self.items[it].top {
                visit(self, it, &mut seen);
            }
        }
        for it in 0..self.items.len() {
            if self.items[it].ups.is_empty() {
                visit(self, it, &mut seen);
            }
        }
        for it in 0..self.items.len() {
            visit(self, it, &mut seen);
        }
        self.renumber();
        let mut best = self.ranks.clone();
        let mut best_x = self.crossings();
        for iter in 0..24 {
            if iter % 2 == 0 {
                for r in self.lo + 1..=self.hi {
                    self.sort_rank(r, true);
                }
            } else {
                let mut r = self.hi - 1;
                while r >= self.lo {
                    self.sort_rank(r, false);
                    r -= 1;
                }
            }
            let x = self.crossings();
            if x < best_x {
                best_x = x;
                best = self.ranks.clone();
            }
            if best_x == 0 {
                break;
            }
        }
        self.ranks = best;
        self.renumber();
    }

    fn renumber(&mut self) {
        for rk in self.ranks.values() {
            for (i, &it) in rk.iter().enumerate() {
                self.items[it].order = i as i64;
            }
        }
    }

    fn sort_rank(&mut self, r: i64, up: bool) {
        let Some(mut rk) = self.ranks.remove(&r) else { return };
        let keys: HashMap<usize, f64> = rk
            .iter()
            .map(|&it| {
                let item = &self.items[it];
                let segs = if up { &item.ups } else { &item.downs };
                if segs.is_empty() {
                    return (it, item.order as f64);
                }
                let mut sum = 0.0;
                for &s in segs {
                    let o = if up { self.segs[s].a } else { self.segs[s].b };
                    sum += self.items[o].order as f64;
                }
                (it, sum / segs.len() as f64)
            })
            .collect();
        rk.sort_by(|a, b| keys[a].partial_cmp(&keys[b]).unwrap_or(std::cmp::Ordering::Equal));
        for (i, &it) in rk.iter().enumerate() {
            self.items[it].order = i as i64;
        }
        self.ranks.insert(r, rk);
    }

    fn crossings(&self) -> i64 {
        let mut n = 0;
        for r in self.lo..self.hi {
            let segs: Vec<usize> = self.rank_items(r).iter().flat_map(|&it| self.items[it].downs.iter().copied()).collect();
            for i in 0..segs.len() {
                for j in i + 1..segs.len() {
                    let (a, b) = (&self.segs[segs[i]], &self.segs[segs[j]]);
                    let (aa, ab, ba, bb) = (self.items[a.a].order, self.items[a.b].order, self.items[b.a].order, self.items[b.b].order);
                    if (aa - ba) * (ab - bb) < 0 {
                        n += 1;
                    }
                }
            }
        }
        n
    }

    /// Gives edges that would collide at a node their own spot on its
    /// border: reversed edges (going back up) move off the centre, so they
    /// don't merge with the edges flowing down.
    fn assign_ports(&mut self) {
        for i in 0..self.items.len() {
            if self.items[i].kind != Kind::Node {
                continue;
            }
            let mut ports = HashMap::new();
            for side in [self.items[i].ups.clone(), self.items[i].downs.clone()] {
                // Segments that need a spot of their own: edges going back
                // up (so they don't merge with the flow) and, when several
                // edges end here with a marker (◆ ◇ △), each of those.
                let headed = |s: usize| {
                    let le = &self.edges[self.segs[s].le];
                    let (a_head, b_head) = self.ends(le);
                    let e = self.edge(le);
                    let (mut a_label, mut b_label) = (&e.from_label, &e.to_label);
                    if le.a_is_to {
                        std::mem::swap(&mut a_label, &mut b_label);
                    }
                    le.a == i && (a_head != Head::None || !a_label.is_empty()) || le.b == i && (b_head != Head::None || !b_label.is_empty())
                };
                let mut own: Vec<usize> = vec![];
                for &s in &side {
                    // A marked edge shares no spot with others (its marker
                    // would seem to belong to them too).
                    if self.edges[self.segs[s].le].rev || headed(s) && side.len() > 1 {
                        own.push(s);
                    }
                }
                if own.is_empty() {
                    continue;
                }
                // Spread them evenly, in the order of their other ends, so
                // they don't cross on the way out.
                own.sort_by_key(|&s| self.items[self.other(s, i)].order);
                let it = &self.items[i];
                let (blo, bo) = (self.box_lo(it), self.box_o(it));
                let (lo, hi) = (blo + 1, blo + bo - 2);
                let c = blo + bo / 2;
                let shared = own.len() < side.len();
                let step = 2.max((hi - lo + 1) / (own.len() as i64 + 1));
                for (k, &s) in own.iter().enumerate() {
                    let mut p = lo - 1 + step * (k as i64 + 1);
                    if shared && p == c {
                        p += 1; // leave the centre to the edges that share it
                    }
                    ports.insert(s, lo.max(hi.min(p)));
                }
            }
            self.items[i].ports = ports;
        }
    }

    /// The item at a segment's far end from it.
    fn other(&self, s: usize, it: usize) -> usize {
        let s = &self.segs[s];
        if s.a == it {
            s.b
        } else {
            s.a
        }
    }

    /// Where the drawn box starts across.
    fn box_lo(&self, it: &Item) -> i64 {
        if self.lr() {
            it.box_y
        } else {
            it.box_x
        }
    }

    /// The drawn box's size across.
    fn box_o(&self, it: &Item) -> i64 {
        if it.box_w == 0 {
            return it.osize;
        }
        if self.lr() {
            it.box_h
        } else {
            it.box_w
        }
    }

    /// Where along the item (across the flow) a segment attaches.
    pub fn port_off(&self, i: usize, s: usize) -> i64 {
        let it = &self.items[i];
        match it.kind {
            Kind::Node => {
                if let Some(&o) = it.ports.get(&s) {
                    return o;
                }
                self.box_lo(it) + self.box_o(it) / 2
            }
            Kind::Cluster => {
                let le = &self.edges[self.segs[s].le];
                let real = (le.a == i && le.a_real) || (le.b == i && le.b_real);
                if !real {
                    if let Some(&o) = it.sub.as_ref().unwrap().port_pos.get(&le.idx) {
                        return o;
                    }
                }
                it.osize / 2
            }
            Kind::Label => it.osize / 2,
            _ => 0,
        }
    }

    fn sep(&self, a: usize, b: usize) -> i64 {
        let (a, b) = (self.items[a].kind, self.items[b].kind);
        let big = |k: Kind| k == Kind::Node || k == Kind::Cluster;
        if self.lr() {
            return 1;
        }
        if big(a) && big(b) {
            return 3;
        }
        if a == Kind::Dummy && b == Kind::Dummy {
            return 1;
        }
        2
    }

    /// Assigns positions across the flow: pack, then pull each item towards
    /// its neighbours, most important first (long edges stay straight).
    fn place(&mut self) {
        for r in self.lo..=self.hi {
            let mut x = 0;
            for &it in self.rank_items(r).to_vec().iter() {
                self.items[it].pos = x;
                x += self.items[it].osize + self.sep(it, it);
            }
        }
        for _ in 0..6 {
            for r in self.lo + 1..=self.hi {
                self.place_rank(r, true);
            }
            let mut r = self.hi - 1;
            while r >= self.lo {
                self.place_rank(r, false);
                r -= 1;
            }
        }
        // Normalise: nothing left of zero, then leave room for the frame.
        let mut min_pos = INF;
        for rk in self.ranks.values() {
            for &it in rk {
                min_pos = min_pos.min(self.items[it].pos);
            }
        }
        let margin = match (self.framed, self.lr()) {
            (false, _) => 0,
            (true, true) => 1,
            (true, false) => 2,
        };
        let all: Vec<usize> = self.ranks.values().flatten().copied().collect();
        for it in all {
            self.items[it].pos += margin - min_pos;
        }
    }

    fn place_rank(&mut self, r: i64, up: bool) {
        let rk = self.rank_items(r).to_vec();
        if rk.is_empty() {
            return;
        }
        let mut desired = vec![0i64; rk.len()];
        let mut prio = vec![0i64; rk.len()];
        for (i, &it) in rk.iter().enumerate() {
            let item = &self.items[it];
            let segs = if up { &item.ups } else { &item.downs };
            desired[i] = item.pos;
            if !segs.is_empty() {
                let (mut lo, mut hi) = (INF, -INF);
                for &s in segs {
                    let other = if up { self.segs[s].a } else { self.segs[s].b };
                    let p = self.items[other].pos + self.port_off(other, s) - self.port_off(it, s);
                    lo = lo.min(p);
                    hi = hi.max(p);
                }
                desired[i] = (lo + hi) / 2;
            }
            prio[i] = match item.kind {
                Kind::Dummy | Kind::Label | Kind::Port => 1000 + segs.len() as i64,
                _ => (item.ups.len() + item.downs.len()) as i64,
            };
        }
        let mut idx: Vec<usize> = (0..rk.len()).collect();
        idx.sort_by(|&a, &b| prio[b].cmp(&prio[a]));
        let mut placed = vec![false; rk.len()];
        for &i in &idx {
            let (mut lo, mut hi) = (-INF, INF);
            // Room needed between i and each placed neighbour.
            let mut need = 0;
            for j in (0..i).rev() {
                need += self.items[rk[j]].osize + self.sep(rk[j], rk[j + 1]);
                if placed[j] {
                    lo = lo.max(self.items[rk[j]].pos + need);
                    break;
                }
            }
            need = 0;
            for j in i + 1..rk.len() {
                need += self.items[rk[j - 1]].osize + self.sep(rk[j - 1], rk[j]);
                if j - 1 == i {
                    need = self.items[rk[i]].osize + self.sep(rk[i], rk[j]);
                }
                if placed[j] {
                    hi = hi.min(self.items[rk[j]].pos - need);
                    break;
                }
            }
            let mut p = desired[i];
            if p < lo {
                p = lo;
            }
            if p > hi && hi >= lo {
                p = hi;
            }
            self.items[rk[i]].pos = p;
            placed[i] = true;
        }
        for i in 1..rk.len() {
            let min = self.items[rk[i - 1]].pos + self.items[rk[i - 1]].osize + self.sep(rk[i - 1], rk[i]);
            if self.items[rk[i]].pos < min {
                self.items[rk[i]].pos = min;
            }
        }
    }

    // ---- routing ----

    pub fn stubs(&self) -> (i64, i64, i64) {
        if self.lr() {
            (2, 2, 2)
        } else {
            (1, 1, 1)
        }
    }

    fn route(&mut self) {
        self.r_size = HashMap::new();
        for r in self.lo..=self.hi {
            let m = self.rank_items(r).iter().map(|&it| self.items[it].rsize).max().unwrap_or(0);
            self.r_size.insert(r, m);
        }
        self.tracks = HashMap::new();
        self.gap_len = HashMap::new();
        let (stub_a, stub_b, straight) = self.stubs();
        for r in self.lo..self.hi {
            let mut keys: HashMap<(i64, i64), usize> = HashMap::new();
            let mut nets: Vec<Net> = vec![];
            let mut any = false;
            for &it in self.rank_items(r) {
                for &s in &self.items[it].downs {
                    any = true;
                    let x1 = self.items[it].pos + self.port_off(it, s);
                    let key = (self.items[it].order, x1);
                    let ni = *keys.entry(key).or_insert_with(|| {
                        nets.push(Net { segs: vec![], src: x1, drops: vec![], lo: x1, hi: x1, track: 0 });
                        nets.len() - 1
                    });
                    let b = self.segs[s].b;
                    let x2 = self.items[b].pos + self.port_off(b, s);
                    let n = &mut nets[ni];
                    n.segs.push(s);
                    n.drops.push(x2);
                    n.lo = n.lo.min(x2);
                    n.hi = n.hi.max(x2);
                }
            }
            let mut bent: Vec<usize> = (0..nets.len()).filter(|&i| nets[i].lo != nets[i].hi).collect();
            let t = assign_tracks(&mut nets, &mut bent);
            for &n in &bent {
                for &s in &nets[n].segs {
                    self.tracks.insert(s, nets[n].track);
                }
            }
            let len = if t > 0 {
                stub_a + t + stub_b
            } else if any {
                straight
            } else {
                1
            };
            self.gap_len.insert(r, len);
        }
        // Rank coordinates.
        self.r_start = HashMap::new();
        self.gap_at = HashMap::new();
        let mut y = 0;
        for r in self.lo..=self.hi {
            self.r_start.insert(r, y);
            y += self.r_size[&r];
            self.gap_at.insert(r, y);
            if r < self.hi {
                y += self.gap_len[&r];
            }
        }
        self.rr = y;
        for r in self.lo..=self.hi {
            for it in self.rank_items(r).to_vec() {
                self.items[it].r = self.r_start[&r] + (self.r_size[&r] - self.items[it].rsize) / 2;
            }
        }
    }

    /// Works out the level's size, making room for a frame's title.
    fn finish_size(&mut self) {
        self.oo = 0;
        for rk in self.ranks.values() {
            for &it in rk {
                self.oo = self.oo.max(self.items[it].pos + self.items[it].osize);
            }
        }
        if self.framed {
            if self.lr() {
                self.oo += 1;
            } else {
                self.oo += 2;
            }
            let title = crate::text::width(&self.g.cl[self.cl].title) + 6;
            if self.lr() {
                let d = title - self.rr;
                if d > 0 {
                    // Widen by moving the far border out.
                    *self.r_start.entry(self.hi).or_default() += d;
                    for it in self.rank_items(self.hi).to_vec() {
                        self.items[it].r += d;
                    }
                    self.rr = title;
                }
            } else if self.oo < title {
                self.oo = title;
            }
        }
        self.port_pos = HashMap::new();
        for it in &self.items {
            if it.kind == Kind::Port {
                self.port_pos.insert(it.edge_idx, it.pos);
            }
        }
        if self.lr() {
            (self.w, self.h) = (self.rr, self.oo);
        } else {
            (self.w, self.h) = (self.oo, self.rr);
        }
    }

    /// For each end of a level edge, whether an arrowhead (or circle,
    /// cross) is drawn there and which.
    pub fn ends(&self, le: &LEdge) -> (Head, Head) {
        let e = self.edge(le);
        let (to_head, from_head) = (e.head, e.tail);
        let (mut a_head, mut b_head) = if le.a_is_to { (to_head, from_head) } else { (from_head, to_head) };
        if !le.a_real {
            a_head = Head::None;
        }
        if !le.b_real {
            b_head = Head::None;
        }
        (a_head, b_head)
    }
}

fn to_axes(lr: bool, it: &mut Item) {
    if lr {
        (it.osize, it.rsize) = (it.h, it.w);
    } else {
        (it.osize, it.rsize) = (it.w, it.h);
    }
    if it.kind == Kind::Port || it.kind == Kind::Dummy {
        (it.osize, it.rsize) = (1, 0);
        if it.kind == Kind::Port {
            it.rsize = 1;
        }
    }
}

/// Gives each bent net a track (0 = nearest the upper rank) so that nets on
/// one track don't overlap, and a net whose riser stands where another net
/// drops goes above it (so the two never share a line).
fn assign_tracks(nets: &mut [Net], list: &mut [usize]) -> i64 {
    if list.is_empty() {
        return 0;
    }
    // n → nets that must be above n
    let mut above: HashMap<usize, Vec<usize>> = HashMap::new();
    for &a in list.iter() {
        for &b in list.iter() {
            if a != b && nets[b].drops.contains(&nets[a].src) {
                above.entry(b).or_default().push(a);
            }
        }
    }
    list.sort_by(|&i, &j| {
        let (a, b) = (&nets[i], &nets[j]);
        if a.lo != b.lo {
            return a.lo.cmp(&b.lo);
        }
        (b.hi - b.lo).cmp(&(a.hi - a.lo))
    });
    let mut done: HashMap<usize, bool> = HashMap::new();
    let mut by_track: Vec<Vec<usize>> = vec![];
    let mut assigned = 0;
    while assigned < list.len() {
        let mut progress = false;
        for &n in list.iter() {
            if done.get(&n).copied().unwrap_or(false) {
                continue;
            }
            let mut ready = true;
            let mut min_t = 0;
            for &a in above.get(&n).map_or(&[][..], |v| v.as_slice()) {
                if !done.get(&a).copied().unwrap_or(false) {
                    ready = false;
                    break;
                }
                min_t = min_t.max(nets[a].track + 1);
            }
            if !ready {
                continue;
            }
            let mut t = min_t as usize;
            while t < by_track.len() {
                let free = by_track[t].iter().all(|&o| !(nets[n].lo <= nets[o].hi + 1 && nets[o].lo <= nets[n].hi + 1));
                if free {
                    break;
                }
                t += 1;
            }
            while t >= by_track.len() {
                by_track.push(vec![]);
            }
            by_track[t].push(n);
            nets[n].track = t as i64;
            done.insert(n, true);
            assigned += 1;
            progress = true;
        }
        if !progress {
            // A cycle of constraints: drop them for the rest.
            for &n in list.iter() {
                if !done.get(&n).copied().unwrap_or(false) {
                    above.remove(&n);
                }
            }
        }
    }
    by_track.len() as i64
}
