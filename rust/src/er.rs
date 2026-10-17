//! Entity relationship diagrams: entities are boxes with their attributes
//! in aligned columns (type, name, keys, comment); relationships are edges
//! labelled with their verb, with each end's cardinality written as 1,
//! 0..1, 0..* or 1..* (crow's feet don't fit in a character cell).
//! Identifying relationships (--) are solid, non-identifying ones (..)
//! dotted.

use std::collections::HashMap;

use crate::graph::{dir_of, Edge, Graph, LineStyle, ROOT};
use crate::layout::fit_graph;
use crate::re::{group, lazy_re};
use crate::text::{label, pad_right, width};
use crate::{Drawn, Error, Options};

lazy_re!(ER_REL, r#"^("[^"]+"|[\w\-.]+)\s*(\|o|\|\||\}o|\}\||o\||o\{|\|\{)(--|\.\.)(o\||\|\||o\{|\|\{|\|o|\}o|\}\|)\s*("[^"]+"|[\w\-.]+)\s*:\s*(.*)$"#);
lazy_re!(ER_ENTITY, r#"^("[^"]+"|[\w\-.]+)(?:\[("?)([^\]]*?)("?)\])?\s*(\{)?\s*$"#);
lazy_re!(ER_ATTR, r#"^(\S+)\s+(\S+)(?:\s+((?:PK|FK|UK)(?:\s*,\s*(?:PK|FK|UK))*))?(?:\s+"([^"]*)")?\s*$"#);

fn cardinality(tok: &str) -> String {
    match tok {
        "||" => "1",
        "|o" | "o|" => "0..1",
        "}o" | "o{" => "0..*",
        "}|" | "|{" => "1..*",
        _ => "",
    }
    .to_string()
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let g = parse_er(rest)?;
    Ok((fit_graph(&g, opt), String::new()))
}

fn entity(g: &mut Graph, id: &str) -> String {
    let id = id.trim_matches('"');
    let n = g.node(id, ROOT);
    n.defined = true;
    n.id.clone()
}

fn parse_er(rest: &str) -> Result<Graph, Error> {
    let mut g = Graph::new("TB");
    let mut attrs: HashMap<String, Vec<[String; 4]>> = HashMap::new();
    let mut open = String::new();
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        if !open.is_empty() {
            if line == "}" {
                open.clear();
                continue;
            }
            if let Some(m) = ER_ATTR.captures(line) {
                let row = [m[1].to_string(), m[2].to_string(), group(&m, 3).replace(' ', ""), group(&m, 4).to_string()];
                attrs.entry(open.clone()).or_default().push(row);
            }
            continue;
        }
        if let Some(d) = line.strip_prefix("direction ") {
            g.dir = dir_of(d.trim());
            continue;
        }
        if let Some(m) = ER_REL.captures(line) {
            let from = entity(&mut g, &m[1]);
            let to = entity(&mut g, &m[5]);
            let mut e = Edge { from, to, from_label: cardinality(&m[2]), to_label: cardinality(&m[4]), ..Default::default() };
            if &m[3] == ".." {
                e.style = LineStyle::Dotted;
            }
            e.label = label(m[6].trim_matches('"'));
            g.edges.push(e);
            continue;
        }
        if let Some(m) = ER_ENTITY.captures(line) {
            let id = entity(&mut g, &m[1]);
            if !group(&m, 3).is_empty() {
                g.nodes.get_mut(&id).unwrap().label = vec![m[3].to_string()];
            }
            if group(&m, 5) == "{" {
                open = id;
            }
        }
    }
    for (id, rows) in &attrs {
        let Some(n) = g.nodes.get_mut(id) else { continue };
        let mut cols = [0i64; 4];
        for r in rows {
            for i in 0..4 {
                cols[i] = cols[i].max(width(&r[i]));
            }
        }
        let mut lines = vec![];
        for r in rows {
            let mut l = format!("{}  {}", pad_right(&r[0], cols[0]), pad_right(&r[1], cols[1]));
            if cols[2] > 0 {
                l += &format!("  {}", pad_right(&r[2], cols[2]));
            }
            if !r[3].is_empty() {
                l += &format!("  {}", r[3]);
            }
            lines.push(l.trim_end_matches(' ').to_string());
        }
        n.sections = vec![lines];
    }
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}
