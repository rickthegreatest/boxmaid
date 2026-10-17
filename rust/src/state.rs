//! State diagrams, drawn with the flowchart layout: states are rounded
//! boxes, [*] a dot (start) or ringed dot (end) per scope, composite states
//! frames, `<<choice>>` a decision and `<<fork>>`/`<<join>>` bars.

use crate::graph::{dir_of, Edge, Graph, Head, LineStyle, Node, Shape, ROOT};
use crate::layout::fit_graph;
use crate::re::{group, lazy_re};
use crate::text::label;
use crate::{Drawn, Error, Options};

lazy_re!(ST_TRANSITION, r"^(\S+?)\s*-->\s*(\S+?)\s*(?::\s*(.*))?$");
lazy_re!(ST_ALIAS, r#"^state\s+"([^"]*)"\s+as\s+(\S+?)\s*(\{)?\s*$"#);
lazy_re!(ST_STEREO, r"^state\s+(\S+?)\s+<<(\w+)>>\s*$");
lazy_re!(ST_COMPOSITE, r"^state\s+(\S+?)\s*\{\s*$");
lazy_re!(ST_DESC, r"^([\w.-]+)\s*:\s*(.+)$");
lazy_re!(ST_NOTE, r"(?i)^note\s+(left|right)\s+of\s+(\S+?)\s*(?::\s*(.*))?$");

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let g = parse_state(rest)?;
    Ok((fit_graph(&g, opt), String::new()))
}

fn state<'g>(g: &'g mut Graph, id: &str, c: usize) -> &'g mut Node {
    let n = g.node(id, c);
    if !n.defined {
        n.shape = Shape::Round;
    }
    n
}

/// [*] means the start (as a source) or the end (as a target) of the scope
/// it appears in.
fn endpoint(g: &mut Graph, id: &str, c: usize, source: bool) -> String {
    if id != "[*]" {
        state(g, id, c);
        return id.to_string();
    }
    let cid = g.cl[c].id.clone();
    let (key, sh) = if source { (format!("\u{0}start:{cid}"), Shape::Start) } else { (format!("\u{0}end:{cid}"), Shape::End) };
    let n = g.node(&key, c);
    n.shape = sh;
    n.defined = true;
    n.label = vec![String::new()];
    key
}

pub(crate) fn parse_state(src: &str) -> Result<Graph, Error> {
    let mut g = Graph::new("TB");
    let mut stack = vec![ROOT];
    let mut notes = 0u32;
    let mut note_open: Option<String> = None;
    for raw in src.split('\n') {
        let line = raw.trim();
        let cur = *stack.last().unwrap();
        if let Some(id) = &note_open {
            if line.eq_ignore_ascii_case("end note") {
                note_open = None;
                continue;
            }
            g.nodes.get_mut(id).unwrap().label.push(line.to_string());
            continue;
        }
        if line.is_empty()
            || line == "--"
            || line.starts_with("hide ")
            || line.starts_with("scale ")
            || line.starts_with("classDef ")
            || line.starts_with("class ")
            || line.starts_with("accTitle")
            || line.starts_with("accDescr")
        {
            continue;
        }
        if line == "}" {
            if stack.len() > 1 {
                stack.pop();
            }
            continue;
        }
        if let Some(d) = line.strip_prefix("direction ") {
            let d = dir_of(d.trim());
            if cur == ROOT {
                g.dir = d;
            } else {
                g.cl[cur].dir = d;
            }
            continue;
        }
        if let Some(m) = ST_ALIAS.captures(line) {
            if group(&m, 3) == "{" {
                let c = g.add_cluster(&m[2], &m[1], cur);
                stack.push(c);
                continue;
            }
            let n = state(&mut g, &m[2], cur);
            n.label = label(&m[1]);
            n.defined = true;
            continue;
        }
        if let Some(m) = ST_STEREO.captures(line) {
            let n = state(&mut g, &m[1], cur);
            n.defined = true;
            match m[2].to_lowercase().as_str() {
                "choice" => {
                    n.shape = Shape::Rhombus;
                    n.label = vec![String::new()];
                }
                "fork" | "join" => {
                    n.shape = Shape::Bar;
                    n.label = vec![String::new()];
                }
                _ => {}
            }
            continue;
        }
        if let Some(m) = ST_COMPOSITE.captures(line) {
            let c = g.add_cluster(&m[1], &m[1], cur);
            stack.push(c);
            continue;
        }
        if let Some(m) = ST_NOTE.captures(line) {
            notes += 1;
            let id = format!("\u{0}note{}", char::from_u32(u32::from(b'a') + notes).unwrap_or('\u{fffd}'));
            let n = g.node(&id, cur);
            n.shape = Shape::Note;
            n.dashed = true;
            n.defined = true;
            n.label = vec![];
            if !group(&m, 3).is_empty() {
                n.label = label(&m[3]);
            } else {
                note_open = Some(id.clone());
            }
            let target = state(&mut g, &m[2], cur).id.clone();
            let mut e = Edge { from: target.clone(), to: id.clone(), style: LineStyle::Dotted, ..Default::default() };
            if m[1].eq_ignore_ascii_case("left") {
                e.from = id;
                e.to = target;
            }
            g.edges.push(e);
            continue;
        }
        if let Some(m) = ST_TRANSITION.captures(line) {
            let from = endpoint(&mut g, &m[1], cur, true);
            let to = endpoint(&mut g, &m[2], cur, false);
            let mut e = Edge { from, to, head: Head::Arrow, ..Default::default() };
            if !group(&m, 3).is_empty() {
                e.label = label(&m[3]);
            }
            g.edges.push(e);
            continue;
        }
        if let Some(m) = ST_DESC.captures(line) {
            let n = state(&mut g, &m[1], cur);
            if !n.defined {
                n.label = vec![];
            }
            n.defined = true;
            n.label.extend(label(&m[2]));
            continue;
        }
        if !line.contains([' ', '\t']) {
            state(&mut g, line, cur);
        }
    }
    // A composite state named by a transition is the frame, not a node.
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}
