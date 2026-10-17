//! C4 diagrams (C4Context, C4Container, …), drawn with the flowchart
//! layout: people, systems and containers are boxes with their type and
//! description, boundaries are frames, relations labelled edges.

use crate::graph::{Edge, Graph, Head, Shape, ROOT};
use crate::layout::fit_graph;
use crate::re::lazy_re;
use crate::text::{label, split_top_level};
use crate::{Drawn, Error, Options};

lazy_re!(C4_CALL, r"^(\w+)\s*\((.*)\)\s*(\{)?\s*$");

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let (g, title) = parse_c4(rest)?;
    Ok((fit_graph(&g, opt), title))
}

fn parse_c4(src: &str) -> Result<(Graph, String), Error> {
    let mut g = Graph::new("TB");
    let mut stack = vec![ROOT];
    let mut title = String::new();
    for raw in src.split('\n') {
        let line = raw.trim();
        let cur = *stack.last().unwrap();
        if line.is_empty() {
            continue;
        }
        if line == "}" {
            if stack.len() > 1 {
                stack.pop();
            }
            continue;
        }
        if let Some(t) = line.strip_prefix("title ") {
            title = t.trim().to_string();
            continue;
        }
        let Some(m) = C4_CALL.captures(line) else { continue };
        let func = &m[1];
        let args = c4_args(&m[2]);
        let arg = |i: usize| args.get(i).cloned().unwrap_or_default();
        let lower = func.to_lowercase();
        if lower.ends_with("boundary") || is_deployment_node(&lower) {
            let mut t = arg(1);
            let kind = arg(2);
            if !kind.is_empty() {
                t += &format!(" [{kind}]");
            }
            let c = g.add_cluster(&arg(0), &t, cur);
            stack.push(c);
            continue;
        } else if lower.starts_with("rel") || lower.starts_with("birel") {
            let mut e = Edge { from: arg(0), to: arg(1), head: Head::Arrow, ..Default::default() };
            if lower.starts_with("birel") {
                e.tail = Head::Arrow;
            }
            if lower.contains("back") {
                std::mem::swap(&mut e.from, &mut e.to);
            }
            e.label = label(&arg(2));
            let t = arg(3);
            if !t.is_empty() {
                e.label.push(format!("[{t}]"));
            }
            g.edges.push(e);
        } else if lower.starts_with("update") || lower.starts_with("addelementtag") || lower.starts_with("addreltag") {
        } else {
            // Person, System, Container, Component and their _Ext/Db/Queue
            // variants: (alias, label, [technology,] description).
            let s = func.strip_suffix("_Ext").unwrap_or(func);
            let kind = s.strip_suffix("_ext").unwrap_or(s);
            let ext = kind != func;
            let n = g.node(&arg(0), cur);
            n.defined = true;
            n.label = label(&arg(1));
            let (mut tech, mut desc) = (String::new(), arg(2));
            if lower.starts_with("container") || lower.starts_with("component") {
                tech = arg(2);
                desc = arg(3);
            }
            let b = kind.strip_suffix("Db").unwrap_or(kind);
            let base = b.strip_suffix("Queue").unwrap_or(b);
            let mut tag = format!("[{base}");
            if !tech.is_empty() {
                tag += &format!(": {tech}");
            }
            tag.push(']');
            if desc.is_empty() {
                n.detail = vec![tag];
            } else {
                n.detail = std::iter::once(tag).chain(label(&desc)).collect();
            }
            n.dashed = ext;
            if kind.ends_with("Db") {
                n.shape = Shape::Cylinder;
            } else if kind.ends_with("Queue") || lower.starts_with("person") {
                n.shape = Shape::Round;
            }
        }
        if m.get(3).is_some() {
            // Something else opening a block: keep the braces balanced.
            stack.push(cur);
        }
    }
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok((g, title))
}

/// Splits call arguments at commas outside quotes, dropping quotes and
/// named arguments ($tags="…").
fn c4_args(s: &str) -> Vec<String> {
    split_top_level(s, ',').iter().map(|a| a.trim()).filter(|a| !a.starts_with('$')).map(|a| a.trim_matches('"').to_string()).collect()
}

/// Whether a call is a deployment node, which frames what it contains like
/// a boundary does.
fn is_deployment_node(lower: &str) -> bool {
    matches!(lower, "deployment_node" | "deployment_node_l" | "deployment_node_r" | "node" | "node_l" | "node_r")
}
