//! Block diagrams put blocks on the grid their source describes: "columns
//! 3", blocks that span columns ("a:2"), empty space, and nested blocks
//! with columns of their own. Edges run through the gaps between rows,
//! each on its own track.

use std::collections::HashMap;

use crate::canvas::{Canvas, LineStyle};
use crate::graph::Shape;
use crate::re::{group, lazy_re};
use crate::text::{label, max_width, width};
use crate::{Drawn, Error, Options, Role};

#[derive(Default)]
struct Item {
    id: String,
    text: Vec<String>,
    span: i64,
    space: bool,
    shape: Shape,
    /// A nested block's grid.
    children: Option<usize>,
    // placed
    x: i64,
    y: i64,
    w: i64,
    h: i64,
}

#[derive(Default)]
struct Grid {
    columns: i64,
    items: Vec<usize>,
    title: String,
}

lazy_re!(BLOCK_TOK, r#"^([\w-]+)(?:(\(\(|\(\[|\[\(|\[\[|\{\{|\(|\[|\{|>)"?(.*?)"?(\)\)|\]\)|\)\]|\]\]|\}\}|\)|\]|\}))?(?::(\d+))?$"#);
lazy_re!(BLOCK_EDGE, r#"^([\w-]+)\s*(?:--\s*"([^"]*)"\s*)?(-->|---|==>|-\.->)\s*([\w-]+)$"#);

/// Splits on spaces outside brackets and quotes.
fn parse_block_tokens(line: &str) -> Vec<&str> {
    let mut out = vec![];
    let (mut depth, mut quoted, mut start) = (0i64, false, 0usize);
    for (i, r) in line.char_indices() {
        if r == '"' {
            quoted = !quoted;
        } else if quoted {
        } else if "[({".contains(r) {
            depth += 1;
        } else if "])}".contains(r) {
            depth = 0.max(depth - 1);
        } else if r == ' ' && depth == 0 {
            if i > start {
                out.push(&line[start..i]);
            }
            start = i + 1;
        }
    }
    if start < line.len() {
        out.push(&line[start..]);
    }
    out
}

struct Link {
    from: String,
    to: String,
    label: String,
    style: LineStyle,
}

struct Board {
    items: Vec<Item>,
    grids: Vec<Grid>,
    cell_w: i64,
    gap_y: i64,
}

const GAP_X: i64 = 3;

impl Board {
    fn place(&mut self, g: usize, x0: i64, y0: i64) -> (i64, i64) {
        let mut cols = self.grids[g].columns;
        if cols <= 0 {
            cols = self.grids[g].items.iter().map(|&i| self.items[i].span).sum();
        }
        let (mut col, mut row_y, mut row_h, mut max_w) = (0i64, y0, 0i64, 0i64);
        for it in self.grids[g].items.clone() {
            let span = self.items[it].span.min(cols);
            if col + span > cols {
                col = 0;
                row_y += row_h + self.gap_y;
                row_h = 0;
            }
            let x = x0 + col * (self.cell_w + GAP_X);
            let mut w = span * self.cell_w + (span - 1) * GAP_X;
            let mut h = 3;
            if let Some(ch) = self.items[it].children {
                // A blank row under the frame, so an arrow into a block
                // inside has room.
                let (cw, chh) = self.place(ch, x + 2, row_y + 2);
                w = w.max(cw + 4);
                h = chh + 3;
            } else if !self.items[it].space {
                h = self.items[it].text.len() as i64 + 2;
            }
            let item = &mut self.items[it];
            (item.x, item.y, item.w, item.h) = (x, row_y, w, h);
            row_h = row_h.max(h);
            max_w = max_w.max(x + w - x0);
            col += span;
        }
        (max_w, row_y + row_h - y0)
    }

    fn draw(&self, cv: &mut Canvas, g: usize) {
        for &i in &self.grids[g].items {
            let it = &self.items[i];
            if it.space {
                continue;
            }
            if let Some(ch) = it.children {
                cv.draw_box(it.x, it.y, it.w, it.h, LineStyle::Solid, true, false, Role::Cluster, "");
                let title = &self.grids[ch].title;
                if !title.is_empty() {
                    cv.text(it.x + 2, it.y, &format!(" {title} "), Role::ClusterTitle, "", true);
                }
                self.draw(cv, ch);
                continue;
            }
            let round = matches!(it.shape, Shape::Round | Shape::Stadium | Shape::Circle | Shape::Cylinder);
            let role = if it.shape == Shape::Rhombus { Role::Decision } else { Role::Node };
            cv.draw_box(it.x, it.y, it.w, it.h, LineStyle::Solid, round, it.shape == Shape::Subroutine, role, "");
            for (j, l) in it.text.iter().enumerate() {
                cv.text(it.x + (it.w - width(l)) / 2, it.y + 1 + j as i64, l, Role::NodeText, "", true);
            }
        }
    }
}

pub(crate) fn render(_body: &str, rest: &str, opt: &Options) -> Drawn {
    let mut b = Board { items: vec![], grids: vec![Grid::default()], cell_w: 0, gap_y: 3 };
    let mut stack = vec![0usize];
    let mut links: Vec<Link> = vec![];
    let mut all: HashMap<String, usize> = HashMap::new();
    let atoi = |s: &str| s.parse::<i64>().ok();
    for raw in rest.split('\n') {
        let line = raw.trim();
        if line.is_empty() || line.starts_with("style ") || line.starts_with("classDef ") || line.starts_with("class ") {
            continue;
        }
        let cur = *stack.last().unwrap();
        if let Some(n) = line.strip_prefix("columns ") {
            if let Some(n) = atoi(n.trim()) {
                b.grids[cur].columns = n;
            }
            continue;
        }
        if line == "end" {
            if stack.len() > 1 {
                stack.pop();
            }
            continue;
        }
        if let Some(m) = BLOCK_EDGE.captures(line) {
            let style = match &m[3] {
                "==>" => LineStyle::Thick,
                "-.->" => LineStyle::Dotted,
                _ => LineStyle::Solid,
            };
            links.push(Link { from: m[1].to_string(), to: m[4].to_string(), label: group(&m, 2).to_string(), style });
            continue;
        }
        if let Some(spec) = line.strip_prefix("block") {
            // block, block:id, block:id:2, block:id["Title"]:2
            let g = b.grids.len();
            b.grids.push(Grid::default());
            let mut it = Item { span: 1, children: Some(g), ..Default::default() };
            let parts: Vec<&str> = spec.strip_prefix(':').unwrap_or(spec).split(':').collect();
            let idx = b.items.len();
            if !parts[0].is_empty() {
                let mut id = parts[0];
                if let Some(i) = id.find(['[', '(']) {
                    if i > 0 {
                        b.grids[g].title = id[i..].trim_matches(|c| "[]()\"".contains(c)).to_string();
                        id = &id[..i];
                    }
                }
                it.id = id.to_string();
                all.insert(id.to_string(), idx);
            }
            if parts.len() > 1 {
                if let Some(n) = atoi(parts[1]) {
                    it.span = n;
                }
            }
            b.items.push(it);
            b.grids[cur].items.push(idx);
            stack.push(g);
            continue;
        }
        for tok in parse_block_tokens(line) {
            if tok == "space" || tok.starts_with("space:") {
                let mut n = 1;
                if let Some((_, v)) = tok.split_once(':') {
                    n = atoi(v).unwrap_or(0);
                }
                b.items.push(Item { space: true, span: 1.max(n), ..Default::default() });
                b.grids[cur].items.push(b.items.len() - 1);
                continue;
            }
            let Some(m) = BLOCK_TOK.captures(tok) else { continue };
            let mut it = Item { id: m[1].to_string(), text: vec![m[1].to_string()], span: 1, ..Default::default() };
            if !group(&m, 3).is_empty() {
                it.text = label(group(&m, 3));
            }
            it.shape = match group(&m, 2) {
                "(" => Shape::Round,
                "((" => Shape::Circle,
                "([" => Shape::Stadium,
                "[(" => Shape::Cylinder,
                "[[" => Shape::Subroutine,
                "{" => Shape::Rhombus,
                "{{" => Shape::Hexagon,
                ">" => Shape::Asym,
                _ => Shape::Rect,
            };
            if !group(&m, 5).is_empty() {
                it.span = 1.max(atoi(group(&m, 5)).unwrap_or(0));
            }
            let idx = b.items.len();
            all.insert(it.id.clone(), idx);
            b.items.push(it);
            b.grids[cur].items.push(idx);
        }
    }
    if b.grids[0].items.is_empty() {
        return Err(Error::Empty);
    }
    // Measure and place, innermost grids first.
    let cell_w = b.items.iter().filter(|it| it.children.is_none() && !it.space).map(|it| max_width(&it.text) + 4).max().unwrap_or(0);
    b.cell_w = cell_w.max(7);
    let (mut w, mut h) = b.place(0, 0, 0);
    // Each edge leaving a row downwards (or upwards) needs a track in that
    // gap; widen the gaps when there are more than fit, and place again.
    let both = |l: &Link| -> Option<(usize, usize)> { Some((*all.get(&l.from)?, *all.get(&l.to)?)) };
    let mut need: HashMap<i64, i64> = HashMap::new();
    for l in &links {
        if let Some((a, t)) = both(l) {
            let (a, t) = (&b.items[a], &b.items[t]);
            if !(a.y == t.y && a.h == t.h) {
                *need.entry(a.y).or_insert(0) += 1;
            }
        }
    }
    let most = need.values().copied().max().unwrap_or(0);
    if most + 2 > b.gap_y {
        b.gap_y = most + 2;
        (w, h) = b.place(0, 0, 0);
    }
    let mut cv = Canvas::new(w + 2, h + 3 + b.gap_y, opt.ascii);
    b.draw(&mut cv, 0);
    // Edges: straight across when side by side, else down into the gap
    // below the source, along its own track, and into the target.
    let mut track_of: HashMap<i64, i64> = HashMap::new();
    for l in &links {
        let Some((a, t)) = both(l) else { continue };
        let (a, t) = (&b.items[a], &b.items[t]);
        if a.y == t.y && a.h == t.h {
            // same row: side to side
            let y = a.y + a.h / 2;
            let (x1, x2) = if t.x < a.x { (a.x - 1, t.x + t.w) } else { (a.x + a.w, t.x - 1) };
            cv.hline(x1.min(x2), x1.max(x2), y, l.style, Role::Edge, "");
            cv.glyph_at(x2, y, if t.x < a.x { "◄" } else { "►" }, Role::Arrow, "");
            if !l.label.is_empty() {
                cv.text((x1 + x2) / 2 - width(&l.label) / 2, y - 1, &l.label, Role::EdgeLabel, "", false);
            }
            continue;
        }
        let down = t.y > a.y;
        let sx = a.x + a.w / 2;
        let tx = t.x + t.w / 2;
        let tr = track_of.entry(a.y).or_insert(0);
        let (sy, ty, track_y) = if down { (a.y + a.h, t.y - 1, a.y + a.h + 1 + *tr) } else { (a.y - 1, t.y + t.h, a.y - 2 - *tr) };
        *tr += 1;
        cv.vline(sx, sy, track_y, l.style, Role::Edge, "");
        cv.hline(sx, tx, track_y, l.style, Role::Edge, "");
        cv.vline(tx, track_y, ty, l.style, Role::Edge, "");
        cv.glyph_at(tx, ty, if down { "▼" } else { "▲" }, Role::Arrow, "");
        if !l.label.is_empty() {
            // beside the line where it leaves the block
            let lw = width(&l.label);
            if cv.blank(sx + 1, sy, lw + 2) {
                cv.text(sx + 2, sy, &l.label, Role::EdgeLabel, "", false);
            } else if cv.blank(tx + 1, ty, lw + 2) {
                cv.text(tx + 2, ty, &l.label, Role::EdgeLabel, "", false);
            }
        }
    }
    Ok((cv.picture(), String::new()))
}
