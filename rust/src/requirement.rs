//! Requirement diagrams and architecture diagrams, drawn with the graph
//! layout.

use crate::graph::{dir_of, Cluster, Edge, Graph, Head, Shape, ROOT};
use crate::layout::fit_graph;
use crate::re::{group, lazy_re};
use crate::text::or_str;
use crate::{Drawn, Error, Options};

lazy_re!(
    REQ_BLOCK,
    r"^(requirement|functionalRequirement|interfaceRequirement|performanceRequirement|physicalRequirement|designConstraint|element)\s+(\S+?)\s*\{\s*$"
);
lazy_re!(REQ_FIELD, r"^(\w+)\s*:\s*(.+)$");
lazy_re!(REQ_REL_FWD, r"^(\S+)\s+-\s*(\w+)\s*->\s+(\S+)$");
lazy_re!(REQ_REL_BWD, r"^(\S+)\s+<-\s*(\w+)\s*-\s+(\S+)$");

fn req_type_name(t: &str) -> &'static str {
    match t {
        "requirement" => "Requirement",
        "functionalRequirement" => "Functional Requirement",
        "interfaceRequirement" => "Interface Requirement",
        "performanceRequirement" => "Performance Requirement",
        "physicalRequirement" => "Physical Requirement",
        "designConstraint" => "Design Constraint",
        "element" => "Element",
        _ => "",
    }
}

fn req_field_name(f: &str) -> &'static str {
    match f {
        "id" => "Id",
        "text" => "Text",
        "risk" => "Risk",
        "verifymethod" => "Verification",
        "type" => "Type",
        "docref" => "Doc ref",
        _ => "",
    }
}

pub(crate) fn render_requirement(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let g = parse_requirement(rest)?;
    Ok((fit_graph(&g, opt), String::new()))
}

pub(crate) fn render_architecture(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let g = parse_architecture(rest)?;
    Ok((fit_graph(&g, opt), String::new()))
}

fn parse_requirement(rest: &str) -> Result<Graph, Error> {
    let mut g = Graph::new("TB");
    let mut open: Option<String> = None;
    let mut fields: Vec<String> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        if let Some(id) = &open {
            if line == "}" {
                g.nodes.get_mut(id).unwrap().sections = vec![std::mem::take(&mut fields)];
                open = None;
                continue;
            }
            if let Some(m) = REQ_FIELD.captures(line) {
                let name = or_str(req_field_name(&m[1].to_lowercase()), &m[1]);
                let mut v = m[2].trim().trim_matches('"').to_string();
                if name == "Risk" || name == "Verification" {
                    v = upper_first(&v);
                }
                fields.push(format!("{name}: {v}"));
            }
            continue;
        }
        if let Some(d) = line.strip_prefix("direction ") {
            g.dir = dir_of(d.trim());
            continue;
        }
        if let Some(m) = REQ_BLOCK.captures(line) {
            let n = g.node(&m[2], ROOT);
            n.defined = true;
            n.label = vec![m[2].to_string()];
            n.header = vec![format!("«{}»", req_type_name(&m[1]))];
            open = Some(m[2].to_string());
            continue;
        }
        let (from, kind, to) = if let Some(m) = REQ_REL_FWD.captures(line) {
            (m[1].to_string(), m[2].to_string(), m[3].to_string())
        } else if let Some(m) = REQ_REL_BWD.captures(line) {
            (m[3].to_string(), m[2].to_string(), m[1].to_string())
        } else {
            continue;
        };
        g.node(&from, ROOT);
        g.node(&to, ROOT);
        g.edges.push(Edge { from, to, head: Head::Arrow, label: vec![format!("«{kind}»")], ..Default::default() });
    }
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}

/// v with its first letter in upper case.
fn upper_first(v: &str) -> String {
    let mut c = v.chars();
    match c.next() {
        Some(f) => f.to_uppercase().chain(c).collect(),
        None => String::new(),
    }
}

lazy_re!(ARCH_GROUP, r"^group\s+([\w-]+)(?:\(([^)]*)\))?(?:\[([^\]]*)\])?(?:\s+in\s+([\w-]+))?$");
lazy_re!(ARCH_SERVICE, r"^service\s+([\w-]+)(?:\(([^)]*)\))?(?:\[([^\]]*)\])?(?:\s+in\s+([\w-]+))?$");
lazy_re!(ARCH_JUNCTION, r"^junction\s+([\w-]+)(?:\s+in\s+([\w-]+))?$");
lazy_re!(ARCH_EDGE, r"^([\w-]+)(?:\{group\})?\s*:\s*([LRTB])\s*(<?)--(>?)\s*([LRTB])\s*:\s*([\w-]+)(?:\{group\})?$");

fn parse_architecture(rest: &str) -> Result<Graph, Error> {
    let mut g = Graph::new("TB");
    let (mut horizontal, mut vertical) = (0, 0);
    // (id, parent) pairs, resolved once every group is known.
    let mut groups: Vec<(String, String)> = vec![];
    let mut members: Vec<(String, String)> = vec![];
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        if let Some(m) = ARCH_GROUP.captures(line) {
            let title = or_str(group(&m, 3), &m[1]);
            groups.push((m[1].to_string(), group(&m, 4).to_string()));
            let i = g.cl.len();
            g.cl.push(Cluster { id: m[1].to_string(), title, ..Default::default() });
            g.clusters.insert(m[1].to_string(), i);
        } else if let Some(m) = ARCH_SERVICE.captures(line) {
            let n = g.node(&m[1], ROOT);
            n.defined = true;
            n.label = vec![or_str(group(&m, 3), &m[1])];
            let icon = group(&m, 2);
            if !icon.is_empty() {
                n.header = vec![icon.to_string()];
            }
            match icon {
                "database" => {
                    n.shape = Shape::Cylinder;
                    n.header = vec![];
                }
                "internet" | "cloud" => n.shape = Shape::Round,
                _ => {}
            }
            members.push((m[1].to_string(), group(&m, 4).to_string()));
        } else if let Some(m) = ARCH_JUNCTION.captures(line) {
            let n = g.node(&m[1], ROOT);
            n.defined = true;
            n.shape = Shape::Start;
            n.label = vec![String::new()];
            members.push((m[1].to_string(), group(&m, 2).to_string()));
        } else if let Some(m) = ARCH_EDGE.captures(line) {
            let mut e = Edge { from: m[1].to_string(), to: m[6].to_string(), ..Default::default() };
            if group(&m, 4) == ">" {
                e.head = Head::Arrow;
            }
            if group(&m, 3) == "<" {
                e.tail = Head::Arrow;
            }
            let side = |s: &str| s == "L" || s == "R";
            if side(&m[2]) && side(&m[5]) {
                horizontal += 1;
            } else {
                vertical += 1;
            }
            g.edges.push(e);
        }
    }
    // Nest groups and place services now that every group is known.
    for (id, parent) in &groups {
        let c = g.clusters[id];
        let p = match g.clusters.get(parent) {
            Some(&p) if !parent.is_empty() && !within(&g, p, c) => p,
            _ => ROOT,
        };
        g.cl[c].parent = Some(p);
        g.cl[p].children.push(c);
    }
    for (id, parent) in &members {
        if parent.is_empty() {
            continue;
        }
        if let Some(&c) = g.clusters.get(parent) {
            g.cl[ROOT].nodes.retain(|x| x != id);
            g.nodes.get_mut(id).unwrap().cluster = c;
            g.cl[c].nodes.push(id.clone());
        }
    }
    if horizontal > vertical {
        g.dir = "LR".to_string();
    }
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}

/// Whether c is p or one of its ancestors so far, which would make putting p
/// inside c a cycle ("group a in b", "group b in a").
fn within(g: &Graph, p: usize, c: usize) -> bool {
    let mut cur = Some(p);
    while let Some(k) = cur {
        if k == c {
            return true;
        }
        cur = g.cl[k].parent;
    }
    false
}
