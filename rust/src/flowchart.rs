//! Flowcharts (and graphs).

use std::collections::HashMap;

use crate::graph::{dir_of, Edge, Graph, Head, LineStyle, Shape, ROOT};
use crate::layout::fit_graph;
use crate::re::{group, lazy_re};
use crate::text::{label, split_top_level};
use crate::{Drawn, Error, Options};

lazy_re!(FLOW_HEADER, r"^\s*(?:flowchart|graph)(?:-elk)?\b\s*(TB|TD|BT|LR|RL|<|>|\^|v)?\s*;?\s*(.*)$");
lazy_re!(EDGE_PLAIN, r"^(<|o|x)?(?:(-{2,})|(={2,})|(-\.+-)|(~{3,}))(>|o|x)?");
lazy_re!(EDGE_OPEN, r"^(<|o|x)?(--|==|-\.)\s");
lazy_re!(CLOSE_DASH, r"(-{2,})([>ox])|(-{3,})");
lazy_re!(CLOSE_EQ, r"(={2,})([>ox])|(={3,})");
lazy_re!(CLOSE_DOT, r"(\.+-)([>ox])?");
lazy_re!(PIPE_LABEL, r"^\s*\|([^|]*)\|");
lazy_re!(EDGE_ID, r"^[A-Za-z0-9_]+@");
lazy_re!(META_STMT, r"^([A-Za-z0-9_]+)@\{(.*)\}$");
lazy_re!(STYLE_KV, r"(stroke|fill|color)\s*:\s*(#[0-9a-fA-F]{3,8}|[a-zA-Z]+)");

pub(crate) fn render(body: &str, _rest: &str, opt: &Options) -> Drawn {
    let g = parse_flowchart(body)?;
    Ok((fit_graph(&g, opt), String::new()))
}

/// Reads a flowchart (or graph) definition.
pub(crate) fn parse_flowchart(src: &str) -> Result<Graph, Error> {
    let mut g: Option<Graph> = None;
    let mut stack: Vec<usize> = vec![];
    let mut class_color: HashMap<String, String> = HashMap::new();
    let mut styled: HashMap<String, String> = HashMap::new();
    for raw in src.split('\n') {
        for st in split_statements(raw) {
            let mut st = st.trim().to_string();
            if st.is_empty() {
                continue;
            }
            if g.is_none() {
                let Some(m) = FLOW_HEADER.captures(&st) else {
                    return Err(Error::Parse(format!("not a flowchart: {st:?}")));
                };
                g = Some(Graph::new(&dir_of(group(&m, 1))));
                stack = vec![ROOT];
                let after = group(&m, 2).to_string();
                if after.trim().is_empty() {
                    continue;
                }
                st = after;
            }
            let gr = g.as_mut().unwrap();
            let cur = *stack.last().unwrap();
            let (word, rest) = st.split_once(' ').unwrap_or((st.as_str(), ""));
            let rest = rest.trim();
            match word {
                "subgraph" => {
                    let (id, title) = subgraph_head(rest);
                    let c = gr.add_cluster(&id, &title, cur);
                    stack.push(c);
                    continue;
                }
                "end" => {
                    if stack.len() > 1 {
                        stack.pop();
                    }
                    continue;
                }
                "direction" => {
                    if cur == ROOT {
                        gr.dir = dir_of(rest.trim());
                    } else {
                        gr.cl[cur].dir = dir_of(rest.trim());
                    }
                    continue;
                }
                "classDef" => {
                    let (names, props) = rest.split_once(' ').unwrap_or((rest, ""));
                    let c = color_of(props);
                    if !c.is_empty() {
                        for n in names.split(',') {
                            class_color.insert(n.trim().to_string(), c.clone());
                        }
                    }
                    continue;
                }
                "class" => {
                    let f: Vec<&str> = rest.split_whitespace().collect();
                    if f.len() >= 2 {
                        for id in f[0].split(',') {
                            // Classes may be assigned before the node appears.
                            gr.node(id.trim(), cur).classes.push(f[1].to_string());
                        }
                    }
                    continue;
                }
                "style" => {
                    let (id, props) = rest.split_once(' ').unwrap_or((rest, ""));
                    let c = color_of(props);
                    if !c.is_empty() {
                        styled.insert(id.to_string(), c);
                    }
                    continue;
                }
                "linkStyle" | "click" | "accTitle" | "accDescr" | "accTitle:" | "accDescr:" | "callback" | "interpolate" => continue,
                _ => {}
            }
            if edge_meta(gr, &st) {
                continue;
            }
            parse_chain(gr, cur, &st);
        }
    }
    let Some(mut g) = g else { return Err(Error::Empty) };
    for n in g.nodes.values_mut() {
        for c in &n.classes {
            if let Some(col) = class_color.get(c) {
                n.color = col.clone();
            }
        }
        if let Some(col) = styled.get(&n.id) {
            n.color = col.clone();
        }
    }
    g.finish();
    if g.nodes.is_empty() && g.clusters.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}

/// Splits a line at semicolons outside quotes and brackets.
fn split_statements(line: &str) -> Vec<String> {
    let mut out = vec![];
    let (mut depth, mut quoted, mut start) = (0i64, false, 0usize);
    for (i, r) in line.char_indices() {
        if r == '"' {
            quoted = !quoted;
        } else if quoted {
        } else if "[({".contains(r) {
            depth += 1;
        } else if "])}".contains(r) {
            depth = (depth - 1).max(0);
        } else if r == ';' && depth == 0 {
            out.push(line[start..i].to_string());
            start = i + 1;
        }
    }
    out.push(line[start..].to_string());
    out
}

/// Reads "id [title]", "id[title]", "\"title\"" or "some title".
fn subgraph_head(s: &str) -> (String, String) {
    let s = s.trim();
    if let Some(i) = s.find(|c| "[(\"{".contains(c)) {
        if i > 0 && s.as_bytes()[i] != b'"' {
            let id = s[..i].trim().to_string();
            let title = label(s[i..].trim_matches(|c| "[](){}".contains(c))).join(" ");
            return (id, title);
        }
    }
    let title = label(s).join(" ");
    (title.clone(), title)
}

/// The colour a node is drawn in from Mermaid style props: the stroke if
/// there is one, else the fill.
pub(crate) fn color_of(props: &str) -> String {
    let (mut stroke, mut fill) = (String::new(), String::new());
    for m in STYLE_KV.captures_iter(props) {
        let c = norm_color(&m[2]);
        match &m[1] {
            "stroke" => stroke = c,
            "fill" => fill = c,
            _ => {}
        }
    }
    // Strokes are often plain black or white outlines; the fill says more.
    if !stroke.is_empty() && !greyish(&stroke) {
        return stroke;
    }
    if !fill.is_empty() && !greyish(&fill) {
        return fill;
    }
    String::new()
}

fn norm_color(c: &str) -> String {
    let c = c.to_lowercase();
    let named = match c.as_str() {
        "red" => "#e5484d",
        "green" => "#30a46c",
        "blue" => "#3e63dd",
        "orange" => "#f76b15",
        "yellow" | "gold" => "#ffc53d",
        "purple" | "violet" => "#8e4ec6",
        "pink" | "magenta" => "#d6409f",
        "cyan" => "#00a2c7",
        "teal" => "#12a594",
        "lightblue" => "#7cc4fa",
        "lightgreen" => "#7ee0a1",
        _ => "",
    };
    if !named.is_empty() {
        return named.to_string();
    }
    let Some(h) = c.strip_prefix('#') else { return String::new() };
    let b = h.as_bytes();
    match b.len() {
        3 | 4 => String::from_utf8(vec![b'#', b[0], b[0], b[1], b[1], b[2], b[2]]).unwrap(),
        6 | 8 => format!("#{}", &h[..6]),
        _ => String::new(),
    }
}

/// Greyish colours (black, white, greys) carry no meaning as a node colour.
fn greyish(c: &str) -> bool {
    if c.len() != 7 {
        return true;
    }
    let p = |i: usize| i64::from_str_radix(&c[i..i + 2], 16).unwrap_or(0);
    let (r, g, b) = (p(1), p(3), p(5));
    r.max(g).max(b) - r.min(g).min(b) < 24
}

/// Whether a statement only styles an edge ("e1@{ animate: true }"): one
/// about a known edge id, or with only edge keys. Such statements aren't
/// nodes.
fn edge_meta(g: &Graph, st: &str) -> bool {
    let Some(m) = META_STMT.captures(st) else { return false };
    if g.edge_ids.contains(&m[1]) {
        return true;
    }
    let mut keys = 0;
    for kv in split_top_level(&m[2], ',') {
        let k = kv.split_once(':').map_or(kv.as_str(), |(k, _)| k);
        match k.trim() {
            "animate" | "animation" | "curve" => keys += 1,
            "" => {}
            _ => return false,
        }
    }
    keys > 0
}

/// Reads "A[x] --> B & C -.-> D".
fn parse_chain(g: &mut Graph, c: usize, s: &str) {
    let mut p = ChainParser { g, c, s, i: 0 };
    let mut left = p.node_group();
    if left.is_empty() {
        return;
    }
    loop {
        p.space();
        if p.done() {
            return;
        }
        let Some(e) = p.edge_op() else { return };
        let right = p.node_group();
        if right.is_empty() {
            return;
        }
        for l in &left {
            for r in &right {
                let mut x = e.clone();
                x.from = l.clone();
                x.to = r.clone();
                p.g.edges.push(x);
            }
        }
        left = right;
    }
}

struct ChainParser<'a> {
    g: &'a mut Graph,
    c: usize,
    s: &'a str,
    i: usize,
}

impl<'a> ChainParser<'a> {
    fn done(&self) -> bool {
        self.i >= self.s.len()
    }
    fn rest(&self) -> &'a str {
        &self.s[self.i..]
    }
    fn byte(&self) -> u8 {
        self.s.as_bytes()[self.i]
    }
    fn space(&mut self) {
        while self.i < self.s.len() && (self.byte() == b' ' || self.byte() == b'\t') {
            self.i += 1;
        }
    }

    fn node_group(&mut self) -> Vec<String> {
        let mut ids = vec![];
        loop {
            let id = self.node_ref();
            if id.is_empty() {
                return ids;
            }
            ids.push(id);
            self.space();
            if !self.rest().starts_with('&') {
                return ids;
            }
            self.i += 1;
        }
    }

    /// Whether an edge starts at the cursor.
    fn at_edge(&self) -> bool {
        let r = self.rest();
        if EDGE_OPEN.is_match(r) {
            return true;
        }
        let Some(m) = EDGE_PLAIN.captures(r) else { return false };
        // o-- and x-- only start an edge after a space ("A o--o B").
        if group(&m, 1) == "o" || group(&m, 1) == "x" {
            return self.i == 0 || self.s.as_bytes()[self.i - 1] == b' ';
        }
        // A bare "--" or "==" is only half an edge.
        !(group(&m, 6).is_empty() && (group(&m, 2) == "--" || group(&m, 3) == "=="))
    }

    fn node_ref(&mut self) -> String {
        self.space();
        let start = self.i;
        while self.i < self.s.len() {
            let r = self.byte();
            if r == b' ' || r == b'\t' || b"[({>&;|\"".contains(&r) || self.rest().starts_with(":::") || self.rest().starts_with("@{") {
                break;
            }
            if self.i > start && self.at_edge() {
                break;
            }
            if r >= 0x80 {
                // Multi-byte: part of an identifier.
                self.i += self.rest().chars().next().map_or(1, char::len_utf8);
                continue;
            }
            self.i += 1;
        }
        let id = self.s[start..self.i].to_string();
        if id.is_empty() {
            return id;
        }
        let c = self.c;
        self.g.node(&id, c);
        if self.rest().starts_with("@{") {
            if let Some(end) = self.rest().find('}') {
                if end > 0 {
                    let data = self.rest()[2..end].to_string();
                    shape_data(self.g.nodes.get_mut(&id).unwrap(), &data);
                    self.i += end + 1;
                }
            }
        }
        self.shape(&id);
        if self.rest().starts_with(":::") {
            self.i += 3;
            let cs = self.i;
            while self.i < self.s.len() {
                let Some(ch) = self.rest().chars().next() else { break };
                if ch.is_alphabetic() || ch.is_numeric() || ch == '_' || ch == '-' {
                    self.i += ch.len_utf8();
                } else {
                    break;
                }
            }
            let class = self.s[cs..self.i].to_string();
            self.g.nodes.get_mut(&id).unwrap().classes.push(class);
        }
        id
    }

    fn shape(&mut self, id: &str) {
        let r = self.rest();
        for (open, closes, shapes) in SHAPE_OPENERS {
            if !r.starts_with(open) {
                continue;
            }
            let body = &r[open.len()..];
            let mut text = String::new();
            let mut matched = false;
            let mut shape = Shape::Rect;
            let trimmed = body.trim_start_matches(' ');
            if let Some(stripped) = trimmed.strip_prefix('"') {
                if let Some(q) = stripped.find('"') {
                    text = trimmed[..q + 2].to_string();
                    let after = trimmed[q + 2..].trim_start_matches(' ');
                    for (i, c) in closes.iter().enumerate() {
                        if after.starts_with(c) {
                            shape = shapes[i];
                            self.i += r.len() - after.len() + c.len();
                            matched = true;
                            break;
                        }
                    }
                }
            }
            if !matched {
                let mut best: Option<(usize, usize)> = None;
                for (i, c) in closes.iter().enumerate() {
                    if let Some(k) = body.find(c) {
                        if best.is_none_or(|(b, _)| k < b) {
                            best = Some((k, i));
                        }
                    }
                }
                let Some((k, bi)) = best else { continue };
                text = body[..k].to_string();
                shape = shapes[bi];
                self.i += open.len() + k + closes[bi].len();
            }
            let n = self.g.nodes.get_mut(id).unwrap();
            n.shape = shape;
            n.label = label(&text);
            n.defined = true;
            return;
        }
    }

    /// Reads one edge operator and its label.
    fn edge_op(&mut self) -> Option<Edge> {
        let mut r = self.rest();
        if let Some(m) = EDGE_ID.find(r) {
            let ml = m.as_str().len();
            if r.len() > ml && "-=~<ox".contains(r.as_bytes()[ml] as char) {
                self.g.edge_ids.insert(r[..ml - 1].to_string());
                self.i += ml;
                r = self.rest();
            }
        }
        let mut e = Edge::default();
        if let Some(m) = EDGE_OPEN.captures(r) {
            let opener = group(&m, 2);
            let whole_end = m.get(0).unwrap().end();
            let body = &r[whole_end..];
            let closer_re = match opener {
                "--" => &*CLOSE_DASH,
                "==" => &*CLOSE_EQ,
                _ => &*CLOSE_DOT,
            };
            if let Some(c) = closer_re.find(body) {
                e.label = label(&body[..c.start()]);
                if let Some(t) = m.get(1) {
                    e.tail = head_of(t.as_str());
                }
                let closer = c.as_str();
                e.style = style_of(&format!("{opener}{closer}")).0;
                e.minlen = style_of(closer).1;
                let last = &closer[closer.len() - 1..];
                e.head = head_of(last);
                if last == "-" || last == "=" {
                    e.head = Head::None;
                }
                self.i += whole_end + c.end();
                self.pipe(&mut e);
                return Some(e);
            }
        }
        let m = EDGE_PLAIN.captures(r)?;
        if !self.at_edge() {
            return None;
        }
        let body = format!("{}{}{}{}", group(&m, 2), group(&m, 3), group(&m, 4), group(&m, 5));
        (e.style, e.minlen) = style_of(&body);
        e.tail = head_of(group(&m, 1));
        e.head = head_of(group(&m, 6));
        if !group(&m, 5).is_empty() {
            e.style = LineStyle::Invisible;
        }
        self.i += m.get(0).unwrap().end();
        self.pipe(&mut e);
        Some(e)
    }

    fn pipe(&mut self, e: &mut Edge) {
        if let Some(m) = PIPE_LABEL.captures(self.rest()) {
            e.label = label(&m[1]);
            self.i += m.get(0).unwrap().end();
        }
        if e.label.len() == 1 && e.label[0].is_empty() {
            e.label = vec![];
        }
    }
}

type Opener = (&'static str, &'static [&'static str], &'static [Shape]);

static SHAPE_OPENERS: &[Opener] = &[
    ("(((", &[")))"], &[Shape::DoubleCircle]),
    ("((", &["))"], &[Shape::Circle]),
    ("([", &["])"], &[Shape::Stadium]),
    ("[[", &["]]"], &[Shape::Subroutine]),
    ("[(", &[")]"], &[Shape::Cylinder]),
    ("{{", &["}}"], &[Shape::Hexagon]),
    ("[/", &["/]", "\\]"], &[Shape::LeanRight, Shape::Trapezoid]),
    ("[\\", &["\\]", "/]"], &[Shape::LeanLeft, Shape::TrapezoidAlt]),
    ("(", &[")"], &[Shape::Round]),
    ("[", &["]"], &[Shape::Rect]),
    ("{", &["}"], &[Shape::Rhombus]),
    (">", &["]"], &[Shape::Asym]),
];

/// Reads the newer A@{ shape: …, label: "…" } syntax.
fn shape_data(n: &mut crate::graph::Node, data: &str) {
    n.defined = true;
    for kv in split_top_level(data, ',') {
        let (k, v) = kv.split_once(':').unwrap_or((&kv, ""));
        let (k, v) = (k.trim(), v.trim());
        match k {
            "label" => n.label = label(v),
            "shape" => {
                n.shape = match v.trim_matches('"') {
                    "rounded" => Shape::Round,
                    "stadium" | "pill" => Shape::Stadium,
                    "subroutine" | "fr-rect" => Shape::Subroutine,
                    "cyl" | "database" | "db" => Shape::Cylinder,
                    "circle" | "circ" => Shape::Circle,
                    "dbl-circ" | "double-circle" => Shape::DoubleCircle,
                    "diam" | "diamond" | "decision" | "question" => Shape::Rhombus,
                    "hex" | "hexagon" | "prepare" => Shape::Hexagon,
                    "lean-r" | "in-out" => Shape::LeanRight,
                    "lean-l" | "out-in" => Shape::LeanLeft,
                    "trap-b" => Shape::Trapezoid,
                    "trap-t" => Shape::TrapezoidAlt,
                    "text" => Shape::Text,
                    "odd" => Shape::Asym,
                    "start" | "sm-circ" => Shape::Start,
                    "stop" | "fr-circ" => Shape::End,
                    _ => Shape::Rect,
                }
            }
            _ => {}
        }
    }
}

fn head_of(s: &str) -> Head {
    match s {
        ">" | "<" => Head::Arrow,
        "o" => Head::Circle,
        "x" => Head::Cross,
        _ => Head::None,
    }
}

/// The line style and minimum length (in ranks) of an operator.
fn style_of(op: &str) -> (LineStyle, i64) {
    let count = |c: char| op.matches(c).count() as i64;
    if op.contains('.') {
        (LineStyle::Dotted, count('.').max(1))
    } else if op.contains('=') {
        (LineStyle::Thick, (count('=') - 1).max(1))
    } else if op.contains('~') {
        (LineStyle::Invisible, (count('~') - 2).max(1))
    } else {
        (LineStyle::Solid, (count('-') - 1).max(1))
    }
}
