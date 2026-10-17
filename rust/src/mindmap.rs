//! Mind maps: the root in a box, its branches as a tree below it. Shapes
//! (square, round, circle, bang, cloud, hexagon) only change how a node is
//! written down in Mermaid; in a tree the text is what matters, so they're
//! shown as plain text, the root boxed.
//!
//! ```text
//! ╭─────────╮
//! │ mindmap │
//! ╰────┬────╯
//!      ├── Origins
//!      │   └── Long history
//!      └── Research
//! ```

use crate::canvas::{Canvas, LineStyle, BIT_DOWN};
use crate::re::{group, lazy_re};
use crate::text::{label, max_width, wrap};
use crate::{Drawn, Error, Options, Role};

lazy_re!(MIND_SHAPE, r"^[^\[\(\{\)]*?(\(\(|\(-|\)\)|\(|\[|\{\{|\))(.*?)(\)\)|-\)|\(\(|\)|\]|\}\}|\()$");

struct MindNode {
    text: Vec<String>,
    indent: usize,
    children: Vec<usize>,
}

fn mind_text(s: &str) -> Vec<String> {
    let s = s.trim();
    match MIND_SHAPE.captures(s) {
        Some(m) => label(group(&m, 2)),
        None => label(s),
    }
}

struct Line {
    /// Tree glyphs before the text, one per level.
    prefix: Vec<&'static str>,
    text: String,
    depth: i64,
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut nodes: Vec<MindNode> = vec![];
    let mut stack: Vec<usize> = vec![];
    for raw in rest.split('\n') {
        if raw.trim().is_empty() {
            continue;
        }
        let indent = raw.len() - raw.trim_start_matches(' ').len();
        let mut t = raw.trim();
        if t.starts_with("::icon(") || t.starts_with(":::") {
            continue;
        }
        if let Some(i) = t.find(":::") {
            if i > 0 {
                t = t[..i].trim();
            }
        }
        let n = nodes.len();
        nodes.push(MindNode { text: mind_text(t), indent, children: vec![] });
        if n == 0 {
            stack = vec![0];
            continue;
        }
        while stack.len() > 1 && nodes[*stack.last().unwrap()].indent >= indent {
            stack.pop();
        }
        let parent = *stack.last().unwrap();
        nodes[parent].children.push(n);
        stack.push(n);
    }
    if nodes.is_empty() {
        return Err(Error::Empty);
    }
    let text_w = if opt.width > 0 { 12.max(opt.width as i64 - 8) } else { 60 };
    // Lay out the tree as lines first, then draw.
    let mut lines: Vec<Line> = vec![];
    fn walk(nodes: &[MindNode], n: usize, prefix: &[&'static str], depth: i64, text_w: i64, lines: &mut Vec<Line>) {
        let kids = &nodes[n].children;
        for (i, &c) in kids.iter().enumerate() {
            let last_child = i == kids.len() - 1;
            let (branch, cont) = if last_child { ("└── ", "    ") } else { ("├── ", "│   ") };
            let text = wrap(&nodes[c].text, 8.max(text_w - 4 * depth));
            for (j, t) in text.into_iter().enumerate() {
                let mut p = prefix.to_vec();
                p.push(if j > 0 { cont } else { branch });
                lines.push(Line { prefix: p, text: t, depth });
            }
            let mut next = prefix.to_vec();
            next.push(cont);
            walk(nodes, c, &next, depth + 1, text_w, lines);
        }
    }
    walk(&nodes, 0, &[], 1, text_w, &mut lines);
    let root_text = wrap(&nodes[0].text, text_w);
    let rn = root_text.len() as i64;
    let box_w = max_width(&root_text) + 4;
    let indent = box_w / 2;
    let mut cv = Canvas::new(indent + text_w + box_w + 8, rn + lines.len() as i64 + 3, opt.ascii);
    cv.draw_box(0, 0, box_w, rn + 2, LineStyle::Solid, true, false, Role::Node, "");
    for (i, t) in root_text.iter().enumerate() {
        cv.text(2, 1 + i as i64, t, Role::NodeText, "", true);
    }
    if !lines.is_empty() {
        cv.add_bits(indent, rn + 1, BIT_DOWN, LineStyle::Solid, Role::Node, "");
    }
    for (y, l) in (rn + 2..).zip(lines.iter()) {
        let mut x = indent;
        for p in &l.prefix {
            // Tree glyphs as line pieces, so they join cleanly.
            match *p {
                "├── " => {
                    cv.vline(x, y - 1, y + 1, LineStyle::Solid, Role::Edge, "");
                    cv.hline(x, x + 2, y, LineStyle::Solid, Role::Edge, "");
                }
                "└── " => {
                    cv.vline(x, y - 1, y, LineStyle::Solid, Role::Edge, "");
                    cv.hline(x, x + 2, y, LineStyle::Solid, Role::Edge, "");
                }
                "│   " => cv.vline(x, y - 1, y + 1, LineStyle::Solid, Role::Edge, ""),
                _ => {}
            }
            x += 4;
        }
        let (role, bold) = if l.depth == 1 { (Role::NodeText, true) } else { (Role::None, false) };
        cv.text(x, y, &l.text, role, "", bold);
    }
    Ok((cv.picture(), String::new()))
}
