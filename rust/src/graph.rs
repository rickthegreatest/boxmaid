//! The graph flowcharts, state, class, ER, C4, requirement and
//! architecture diagrams parse into, and the layered layout draws.

use std::collections::{HashMap, HashSet};

pub(crate) use crate::canvas::LineStyle;

#[derive(Clone, Copy, PartialEq, Eq, Debug, Default)]
pub(crate) enum Shape {
    #[default]
    Rect,
    Round,
    Stadium,
    Subroutine,
    Cylinder,
    Circle,
    DoubleCircle,
    Asym,
    Rhombus,
    Hexagon,
    LeanRight,
    LeanLeft,
    Trapezoid,
    TrapezoidAlt,
    Start,
    End,
    Bar,
    Note,
    Text,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug, Default)]
pub(crate) enum Head {
    #[default]
    None,
    Arrow,
    Circle,
    Cross,
    Triangle,
    Diamond,
    DiamondOpen,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct Node {
    pub id: String,
    pub label: Vec<String>,
    pub detail: Vec<String>,
    pub header: Vec<String>,
    pub sections: Vec<Vec<String>>,
    pub shape: Shape,
    pub color: String,
    pub dashed: bool,
    pub classes: Vec<String>,
    pub defined: bool,
    pub cluster: usize,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct Edge {
    pub from_label: String,
    pub to_label: String,
    pub from: String,
    pub to: String,
    pub label: Vec<String>,
    pub style: LineStyle,
    pub head: Head,
    pub tail: Head,
    pub minlen: i64,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct Cluster {
    pub id: String,
    pub title: String,
    pub dir: String,
    pub parent: Option<usize>,
    pub children: Vec<usize>,
    pub nodes: Vec<String>,
}

#[derive(Clone, Debug)]
pub(crate) struct Graph {
    pub dir: String,
    pub nodes: HashMap<String, Node>,
    pub order: Vec<String>,
    pub edges: Vec<Edge>,
    /// All clusters; index 0 is the root.
    pub cl: Vec<Cluster>,
    pub clusters: HashMap<String, usize>,
    /// Ids given to edges ("A e1@--> B").
    pub edge_ids: HashSet<String>,
}

pub(crate) const ROOT: usize = 0;

impl Graph {
    pub fn new(dir: &str) -> Graph {
        Graph {
            dir: dir.to_string(),
            nodes: HashMap::new(),
            order: vec![],
            edges: vec![],
            cl: vec![Cluster::default()],
            clusters: HashMap::new(),
            edge_ids: HashSet::new(),
        }
    }

    /// The node with id, created in cluster c when it's new. A node first
    /// mentioned at the top level moves into the first subgraph that
    /// mentions it, like Mermaid does.
    pub fn node(&mut self, id: &str, c: usize) -> &mut Node {
        if !self.nodes.contains_key(id) {
            self.nodes.insert(id.to_string(), Node { id: id.to_string(), label: vec![id.to_string()], cluster: c, ..Default::default() });
            self.order.push(id.to_string());
            self.cl[c].nodes.push(id.to_string());
        } else if self.nodes[id].cluster == ROOT && c != ROOT {
            self.move_node(id, c);
        }
        self.nodes.get_mut(id).unwrap()
    }

    /// Keeps both sides of membership in sync, including redeclarations.
    pub fn move_node(&mut self, id: &str, c: usize) {
        let old = self.nodes[id].cluster;
        if old != c {
            self.cl[old].nodes.retain(|x| x != id);
            self.nodes.get_mut(id).unwrap().cluster = c;
            self.cl[c].nodes.push(id.to_string());
        }
    }

    pub fn add_cluster(&mut self, id: &str, title: &str, parent: usize) -> usize {
        let i = self.cl.len();
        self.cl.push(Cluster { id: id.to_string(), title: title.to_string(), parent: Some(parent), ..Default::default() });
        self.cl[parent].children.push(i);
        if !id.is_empty() {
            self.clusters.insert(id.to_string(), i);
        }
        i
    }

    /// Resolves what can only be known at the end: an edge to a subgraph's
    /// id is an edge to the subgraph, not to a node of that name.
    pub fn finish(&mut self) {
        let ids: Vec<String> = self.clusters.keys().cloned().collect();
        for id in ids {
            if let Some(n) = self.nodes.get(&id) {
                if !n.defined {
                    let c = n.cluster;
                    self.nodes.remove(&id);
                    self.order.retain(|x| *x != id);
                    self.cl[c].nodes.retain(|x| *x != id);
                }
            }
        }
        for e in &mut self.edges {
            if e.minlen < 1 {
                e.minlen = 1;
            }
        }
        // Anonymous subgraphs get an id of their own, in tree order.
        let mut n = 0;
        fn name(g: &mut Graph, c: usize, n: &mut i64) {
            for ch in g.cl[c].children.clone() {
                if g.cl[ch].id.is_empty() {
                    *n += 1;
                    let id = format!("\u{0}cluster{n}");
                    g.cl[ch].id = id.clone();
                    g.clusters.insert(id, ch);
                }
                name(g, ch, n);
            }
        }
        name(self, ROOT, &mut n);
    }
}

pub(crate) fn dir_of(s: &str) -> String {
    match s {
        "LR" | ">" => "LR",
        "RL" | "<" => "RL",
        "BT" | "^" => "BT",
        _ => "TB",
    }
    .to_string()
}
