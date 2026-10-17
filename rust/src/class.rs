//! Class diagrams: classes are boxes with their «annotation», name,
//! attributes and methods; relations are edges with UML heads (hollow
//! triangle for inheritance and realisation, filled diamond for
//! composition, hollow diamond for aggregation, arrow for association and
//! dependency), labels and multiplicities. Namespaces are frames.

use std::collections::HashMap;

use crate::graph::{dir_of, Edge, Graph, Head, LineStyle, Shape, ROOT};
use crate::layout::fit_graph;
use crate::re::{group, lazy_re};
use crate::text::label;
use crate::{Drawn, Error, Options};

lazy_re!(
    CLASS_REL,
    concat!(
        r#"^(\S+?)\s*(?:"([^"]*)")?\s*"#,
        r"(<\|--|<\|\.\.|\*--|\*\.\.|o--|o\.\.|<--|<\.\.|--\|>|\.\.\|>|--\*|\.\.\*|--o|\.\.o|-->|\.\.>|--|\.\.|\(\)--|--\(\))",
        r#"\s*(?:"([^"]*)")?\s*(\S+?)\s*(?::\s*(.*))?$"#
    )
);
lazy_re!(CLASS_DECL, r#"^class\s+([^\s{~\[:]+)(~[^~]+~)?(?:\["([^"]*)"\])?(?::::\S+)?\s*(\{)?\s*(\})?$"#);
lazy_re!(CLASS_MEMBER, r"^([^\s:]+)\s*:\s*(.+)$");
lazy_re!(CLASS_ANNOT, r"^<<\s*([^>]+?)\s*>>\s*(\S+)?$");
lazy_re!(CLASS_NOTE, r#"^note\s+(?:for\s+(\S+)\s+)?"(.*)"$"#);
lazy_re!(GENERIC, r"~([^~]+)~");

#[derive(Default)]
struct ClassInfo {
    attrs: Vec<String>,
    methods: Vec<String>,
    annotations: Vec<String>,
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let g = parse_class(rest)?;
    Ok((fit_graph(&g, opt), String::new()))
}

/// The class named id (with ~generics~), created in cluster c when new;
/// returns its id.
fn class(g: &mut Graph, info: &mut HashMap<String, ClassInfo>, id: &str, c: usize) -> String {
    let id = id.trim();
    let disp = GENERIC.replace_all(id, "<$1>").into_owned();
    let id = GENERIC.replace_all(id, "").into_owned();
    let n = g.node(&id, c);
    if !n.defined {
        n.defined = true;
        n.label = vec![disp];
    }
    info.entry(id.clone()).or_default();
    id
}

fn parse_class(rest: &str) -> Result<Graph, Error> {
    let mut g = Graph::new("TB");
    let mut stack = vec![ROOT];
    let mut info: HashMap<String, ClassInfo> = HashMap::new();
    let mut notes = 0;
    let mut open: Option<String> = None; // inside "class X {"
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let cur = *stack.last().unwrap();
        if let Some(id) = &open {
            if line == "}" {
                open = None;
                continue;
            }
            let ci = info.get_mut(id).unwrap();
            if let Some(m) = CLASS_ANNOT.captures(line) {
                if group(&m, 2).is_empty() {
                    ci.annotations.push(m[1].to_string());
                    continue;
                }
            }
            add_member(ci, line);
            continue;
        }
        if let Some(d) = line.strip_prefix("direction ") {
            g.dir = dir_of(d.trim());
            continue;
        }
        if let Some(n) = line.strip_prefix("namespace ") {
            let name = n.strip_suffix('{').unwrap_or(n).trim();
            let c = g.add_cluster(name, name, cur);
            stack.push(c);
            continue;
        }
        if line == "}" {
            if stack.len() > 1 {
                stack.pop();
            }
            continue;
        }
        if ["classDef ", "cssClass ", "style ", "click ", "link ", "callback ", "accTitle", "accDescr"].iter().any(|p| line.starts_with(p)) {
            continue;
        }
        if let Some(m) = CLASS_DECL.captures(line) {
            let id = class(&mut g, &mut info, &format!("{}{}", &m[1], group(&m, 2)), cur);
            if !group(&m, 3).is_empty() {
                g.nodes.get_mut(&id).unwrap().label = vec![m[3].to_string()];
            }
            if group(&m, 4) == "{" && group(&m, 5).is_empty() {
                open = Some(id);
            }
            continue;
        }
        if let Some(m) = CLASS_ANNOT.captures(line) {
            if !group(&m, 2).is_empty() {
                let id = class(&mut g, &mut info, &m[2], cur);
                info.get_mut(&id).unwrap().annotations.push(m[1].to_string());
                continue;
            }
        }
        if let Some(m) = CLASS_NOTE.captures(line) {
            notes += 1;
            let id = format!("\u{0}note{notes}");
            let n = g.node(&id, cur);
            n.shape = Shape::Note;
            n.dashed = true;
            n.defined = true;
            n.label = label(&m[2]);
            if !group(&m, 1).is_empty() {
                class(&mut g, &mut info, &m[1], cur);
                g.edges.push(Edge { from: id, to: m[1].to_string(), style: LineStyle::Dotted, ..Default::default() });
            }
            continue;
        }
        if let Some(m) = CLASS_REL.captures(line) {
            class(&mut g, &mut info, &m[1], cur);
            class(&mut g, &mut info, &m[5], cur);
            let mut e = Edge {
                from: GENERIC.replace_all(&m[1], "").into_owned(),
                to: GENERIC.replace_all(&m[5], "").into_owned(),
                from_label: group(&m, 2).to_string(),
                to_label: group(&m, 4).to_string(),
                ..Default::default()
            };
            let op = &m[3];
            if op.contains("..") {
                e.style = LineStyle::Dotted;
            }
            e.tail = class_head(&op[..2]);
            e.head = class_head(&op[op.len() - 2..]);
            if !group(&m, 6).is_empty() {
                e.label = label(&m[6]);
            }
            g.edges.push(e);
            continue;
        }
        if let Some(m) = CLASS_MEMBER.captures(line) {
            let id = class(&mut g, &mut info, &m[1], cur);
            add_member(info.get_mut(&id).unwrap(), &m[2]);
            continue;
        }
        if !line.contains([' ', '\t']) {
            class(&mut g, &mut info, line, cur);
        }
    }
    for (id, ci) in &info {
        let Some(n) = g.nodes.get_mut(id) else { continue };
        for a in &ci.annotations {
            n.header.push(format!("«{a}»"));
        }
        if !ci.attrs.is_empty() || !ci.methods.is_empty() {
            n.sections = vec![ci.attrs.clone(), ci.methods.clone()];
        }
    }
    g.finish();
    if g.nodes.is_empty() {
        return Err(Error::Empty);
    }
    Ok(g)
}

/// Files a member as an attribute or (with parentheses) a method, showing
/// generics as <T> and dropping the static/abstract markers.
fn add_member(ci: &mut ClassInfo, m: &str) {
    let m = GENERIC.replace_all(m, "<$1>");
    let m = m.trim();
    if m.is_empty() {
        return;
    }
    let v = m.trim_end_matches(['$', '*']).to_string();
    if m.contains('(') {
        ci.methods.push(v);
    } else {
        ci.attrs.push(v);
    }
}

fn class_head(tok: &str) -> Head {
    match tok {
        "<|" | "|>" => Head::Triangle,
        "*-" | "-*" => Head::Diamond,
        "o-" | "-o" | "o." | ".o" => Head::DiamondOpen,
        "<-" | "<." | "->" | ".>" => Head::Arrow,
        "()" => Head::Circle,
        _ if tok.ends_with('*') || tok.starts_with('*') => Head::Diamond,
        _ => Head::None,
    }
}
